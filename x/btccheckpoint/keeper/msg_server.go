package keeper

import (
	"context"
	"errors"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	ckpttypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	k Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{keeper}
}

// TODO at some point add proper logging of error
// TODO emit some events for external consumers. Those should be probably emitted
// at EndBlockerCallback
func (ms msgServer) InsertBTCSpvProof(ctx context.Context, req *types.MsgInsertBTCSpvProof) (*types.MsgInsertBTCSpvProofResponse, error) {
	// Get the SDK wrapped context
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	rawSubmission, err := types.ParseSubmission(req, ms.k.GetPowLimit(), ms.k.GetExpectedTag(sdkCtx))

	if err != nil {
		return nil, types.ErrInvalidCheckpointProof.Wrap(err.Error())
	}

	submissionKey := rawSubmission.GetSubmissionKey()

	if ms.k.HasSubmission(sdkCtx, submissionKey) {
		return nil, types.ErrDuplicatedSubmission
	}

	newSubmissionOldestHeaderDepth, err := ms.k.GetSubmissionBtcInfo(sdkCtx, submissionKey)

	if err != nil {
		return nil, types.ErrInvalidHeader.Wrap(err.Error())
	}

	epochNum := rawSubmission.CheckpointData.Epoch

	ed := ms.k.GetEpochData(sdkCtx, epochNum)

	if ed == nil {
		// we do not have any data saved yet
		newEd := types.NewEmptyEpochData()
		ed = &newEd
	}

	if ed.Status == types.Finalized {
		// we have already finalized given epoch so we do not need any more submissions
		return nil, types.ErrEpochAlreadyFinalized
	}

	// At this point:
	// - every proof of inclusion is valid i.e every transaction is proved to be
	// part of provided block and contains some OP_RETURN data
	// - header is proved to be part of the chain we know about through BTCLightClient
	// - epoch is not yet finalized
	// - this is new checkpoint submission
	// Verify if this is expected checkpoint
	if err := ms.k.checkpointingKeeper.VerifyCheckpoint(sdkCtx, rawSubmission.CheckpointData); err != nil {
		if errors.Is(err, ckpttypes.ErrConflictingCheckpoint) {
			// We end such transaction with success to preserve setting of the conflict
			// flag in the state. This flag will trigger halt of the chain in the
			// EndBlocker call
			return &types.MsgInsertBTCSpvProofResponse{}, nil
		}

		return nil, err
	}

	if err := ms.k.checkAncestors(sdkCtx, epochNum, newSubmissionOldestHeaderDepth); err != nil {
		return nil, err
	}

	// construct TransactionInfo pair and the submission data
	txsInfo := make([]*types.TransactionInfo, len(submissionKey.Key))
	for i := range submissionKey.Key {
		// creating a per-iteration `txKey` variable rather than assigning it in the `for` statement
		// in order to prevent overwriting previous `txKey`
		// see https://github.com/golang/go/discussions/56010
		txKey := submissionKey.Key[i]
		txsInfo[i] = types.NewTransactionInfo(txKey, req.Proofs[i].BtcTransaction, req.Proofs[i].MerkleNodes)
	}
	submissionData := rawSubmission.GetSubmissionData(epochNum, txsInfo)

	// Everything is fine, save new checkpoint and update Epoch data
	ms.k.addEpochSubmission(
		sdkCtx,
		epochNum,
		ed,
		submissionKey,
		submissionData,
	)

	// At this point, the BTC checkpoint is a valid submission and is
	// not duplicated (first time seeing the pair of BTC txs)
	// Thus, we can safely consider this message as refundable
	ms.k.incentiveKeeper.IndexRefundableMsg(sdkCtx, req)

	return &types.MsgInsertBTCSpvProofResponse{}, nil
}

// UpdateParams updates the params.
func (ms msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.k.authority != req.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.k.authority, req.Authority)
	}
	if err := req.Params.Validate(); err != nil {
		return nil, govtypes.ErrInvalidProposalMsg.Wrapf("invalid parameter: %v", err)
	}

	// CheckpointFinalizationTimeout must remain immutable as changing it
	// breaks a lot of system assumption
	ctx := sdk.UnwrapSDKContext(goCtx)
	if req.Params.CheckpointFinalizationTimeout != ms.k.GetParams(ctx).CheckpointFinalizationTimeout {
		return nil, govtypes.ErrInvalidProposalMsg.Wrapf("the checkpoint finalization timeout cannot be changed")
	}

	if err := ms.k.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
