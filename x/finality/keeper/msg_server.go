package keeper

import (
	"bytes"
	"context"
	"fmt"
	"time"

	errorsmod "cosmossdk.io/errors"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// UpdateParams updates the params
func (ms msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.authority != req.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, req.Authority)
	}
	if err := req.Params.Validate(); err != nil {
		return nil, govtypes.ErrInvalidProposalMsg.Wrapf("invalid parameter: %v", err)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := ms.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

// ResumeFinalityProposal handles the proposal for resuming finality from halting
func (ms msgServer) ResumeFinalityProposal(goCtx context.Context, req *types.MsgResumeFinalityProposal) (*types.MsgResumeFinalityProposalResponse, error) {
	if ms.authority != req.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := ms.HandleResumeFinalityProposal(ctx, req.FpPksHex, req.HaltingHeight); err != nil {
		return nil, govtypes.ErrInvalidProposalMsg.Wrapf("failed to handle resume finality proposal: %v", err)
	}

	return &types.MsgResumeFinalityProposalResponse{}, nil
}

// AddFinalitySig adds a new vote to a given block
func (ms msgServer) AddFinalitySig(goCtx context.Context, req *types.MsgAddFinalitySig) (*types.MsgAddFinalitySigResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyAddFinalitySig)

	if err := req.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	activationHeight, errMod := ms.validateActivationHeight(ctx, req.BlockHeight)
	if errMod != nil {
		return nil, errMod.Wrapf("finality block height: %d is lower than activation height %d", req.BlockHeight, activationHeight)
	}

	indexedBlock, err := ms.GetBlock(ctx, req.BlockHeight)
	if err != nil {
		return nil, err
	}
	should, err := ms.ShouldAcceptSigForHeight(ctx, indexedBlock)
	if err != nil {
		return nil, err
	}
	if !should {
		return nil, types.ErrSigHeightOutdated.Wrapf("height: %d", req.BlockHeight)
	}

	fpPK := req.FpBtcPk

	// ensure the finality provider exists
	fp, err := ms.BTCStakingKeeper.GetFinalityProvider(ctx, req.FpBtcPk.MustMarshal())
	if err != nil {
		return nil, err
	}
	// ensure the finality provider is not slashed at this time point
	// NOTE: it's possible that the finality provider equivocates for height h, and the signature is processed at
	// height h' > h. In this case:
	// - Babylon should reject any new signature from this finality provider, since it's known to be adversarial
	// - Babylon should set its voting power since height h'+1 to be zero, due to the same reason
	// - Babylon should NOT set its voting power between [h, h'] to be zero, since
	//   - Babylon BTC staking ensures safety upon 2f+1 votes, *even if* f of them are adversarial. This is
	//     because as long as a block gets 2f+1 votes, any other block with 2f+1 votes has a f+1 quorum
	//     intersection with this block, contradicting to the assumption and leading to the safety proof.
	//     This ensures slashable safety together with EOTS, thus does not undermine Babylon's security guarantee.
	//   - Due to this reason, when tallying a block, Babylon finalises this block upon 2f+1 votes. If we
	//     modify voting power table in the history, some finality decisions might be contradicting to the
	//     signature set and voting power table.
	//   - To fix the above issue, Babylon has to allow finalise and unfinalise blocks. However, this means
	//     Babylon will lose safety under an adaptive adversary corrupting even 1 finality provider. It can simply
	//     corrupt a new finality provider and equivocate a historical block over and over again, making a previous block
	//     unfinalisable forever
	if fp.IsSlashed() {
		return nil, bstypes.ErrFpAlreadySlashed.Wrapf("finality provider public key: %s", fpPK.MarshalHex())
	}

	if fp.IsJailed() {
		return nil, bstypes.ErrFpAlreadyJailed.Wrapf("finality provider public key: %s", fpPK.MarshalHex())
	}

	// ensure the finality provider has voting power at this height
	if ms.GetVotingPower(ctx, fpPK.MustMarshal(), req.BlockHeight) == 0 {
		return nil, types.ErrInvalidFinalitySig.Wrapf("the finality provider %s does not have voting power at height %d", fpPK.MarshalHex(), req.BlockHeight)
	}

	existingSig, err := ms.GetSig(ctx, req.BlockHeight, fpPK)
	if err == nil && existingSig.Equals(req.FinalitySig) {
		ms.Logger(ctx).Debug("Received duplicated finality vote", "block height", req.BlockHeight, "finality provider", req.FpBtcPk)
		// exactly same vote already exists, return error
		// this is to secure the tx refunding against duplicated messages
		return nil, types.ErrDuplicatedFinalitySig
	}

	// find the timestamped public randomness commitment for this height from this finality provider
	prCommit, err := ms.GetTimestampedPubRandCommitForHeight(ctx, req.FpBtcPk, req.BlockHeight)
	if err != nil {
		return nil, err
	}

	signingContext := signingcontext.FpFinVoteContextV0(ctx.ChainID(), ms.finalityModuleAddress)

	// verify the finality signature message w.r.t. the public randomness commitment
	// including the public randomness inclusion proof and the finality signature
	if err := types.VerifyFinalitySig(req, prCommit, signingContext); err != nil {
		return nil, err
	}
	// the public randomness is good, set the public randomness
	ms.SetPubRand(ctx, req.FpBtcPk, req.BlockHeight, *req.PubRand)

	// verify whether the voted block is a fork or not
	if !bytes.Equal(indexedBlock.AppHash, req.BlockAppHash) {
		// the finality provider votes for a fork!

		// construct evidence
		evidence := &types.Evidence{
			FpBtcPk:              req.FpBtcPk,
			BlockHeight:          req.BlockHeight,
			PubRand:              req.PubRand,
			CanonicalAppHash:     indexedBlock.AppHash,
			CanonicalFinalitySig: nil,
			ForkAppHash:          req.BlockAppHash,
			ForkFinalitySig:      req.FinalitySig,
			SigningContext:       signingContext,
		}

		// if this finality provider has also signed canonical block, slash it
		canonicalSig, err := ms.GetSig(ctx, req.BlockHeight, fpPK)
		if err == nil {
			// set canonial sig
			evidence.CanonicalFinalitySig = canonicalSig

			fpBTCSK, err := evidence.ExtractBTCSK()
			if err != nil {
				return nil, err
			}

			// slash this finality provider, including setting its voting power to
			// zero, extracting its BTC SK, and emit an event
			if err := ms.slashFinalityProvider(ctx, fpBTCSK, evidence); err != nil {
				return nil, err
			}
		}

		// save evidence
		ms.SetEvidence(ctx, evidence)

		// NOTE: we should NOT return error here, otherwise the state change triggered in this tx
		// (including the evidence) will be rolled back
		return &types.MsgAddFinalitySigResponse{}, nil
	}

	// this signature is good, add vote to DB
	ms.SetSig(ctx, req.BlockHeight, fpPK, req.FinalitySig)

	// update `HighestVotedHeight` if needed
	if fp.HighestVotedHeight < uint32(req.BlockHeight) {
		fp.HighestVotedHeight = uint32(req.BlockHeight)
		err := ms.BTCStakingKeeper.UpdateFinalityProvider(ctx, fp)
		if err != nil {
			return nil, fmt.Errorf("failed to update the finality provider: %w", err)
		}
	}

	// if this finality provider has signed the canonical block before,
	// slash it via extracting its secret key, and emit an event
	if ms.HasEvidence(ctx, req.FpBtcPk, req.BlockHeight) {
		// the finality provider has voted for a fork before!
		// If this evidence is at the same height as this signature, slash this finality provider

		// get evidence
		evidence, err := ms.GetEvidence(ctx, req.FpBtcPk, req.BlockHeight)
		if err != nil {
			panic(fmt.Errorf("failed to get evidence despite HasEvidence returns true"))
		}

		// set canonical sig to this evidence
		evidence.CanonicalFinalitySig = req.FinalitySig
		ms.SetEvidence(ctx, evidence)

		fpBTCSK, err := evidence.ExtractBTCSK()
		if err != nil {
			return nil, err
		}

		// slash this finality provider, including setting its voting power to
		// zero, extracting its BTC SK, and emit an event
		if err := ms.slashFinalityProvider(ctx, fpBTCSK, evidence); err != nil {
			return nil, err
		}

		// NOTE: we should NOT return error here, otherwise the state change triggered in this tx
		// (including the evidence and slashing) will be rolled back
		return &types.MsgAddFinalitySigResponse{}, nil
	}

	// at this point, the finality signature is 1) valid, 2) over a canonical block,
	// and 3) not duplicated.
	// Thus, we can safely consider this message as refundable
	ms.IncentiveKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgAddFinalitySigResponse{}, nil
}

func (ms msgServer) ShouldAcceptSigForHeight(ctx context.Context, block *types.IndexedBlock) (bool, error) {
	epochNum := ms.CheckpointingKeeper.GetEpochByHeight(ctx, block.Height)
	lastFinalizedEpoch := ms.GetLastFinalizedEpoch(ctx)
	timestamped := lastFinalizedEpoch >= epochNum

	// should NOT accept sig for height is the block is already and finalized by the BTC-timestamping
	// protocol
	should := !(block.Finalized && timestamped)

	return should, nil
}

// CommitPubRandList commits a list of EOTS public randomness
func (ms msgServer) CommitPubRandList(goCtx context.Context, req *types.MsgCommitPubRandList) (*types.MsgCommitPubRandListResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyCommitPubRandList)

	if err := req.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	// check the commit start height is not too far into the future
	if req.StartHeight >= uint64(ctx.BlockHeader().Height)+types.MaxPubRandCommitOffset {
		return nil, types.ErrInvalidPubRand.Wrapf("start height %d is too far into the future. Current height is %d and max offset is %d", req.StartHeight, ctx.BlockHeader().Height, types.MaxPubRandCommitOffset)
	}

	activationHeight, errMod := ms.validateActivationHeight(ctx, req.StartHeight)
	if errMod != nil {
		return nil, types.ErrFinalityNotActivated.Wrapf(
			"public rand commit start block height: %d is lower than activation height %d",
			req.StartHeight, activationHeight,
		)
	}

	// ensure the request contains enough number of public randomness
	minPubRand := ms.GetParams(ctx).MinPubRand
	givenNumPubRand := req.NumPubRand
	if givenNumPubRand < minPubRand {
		return nil, types.ErrTooFewPubRand.Wrapf("required minimum: %d, actual: %d", minPubRand, givenNumPubRand)
	}
	// TODO: ensure log_2(givenNumPubRand) is an integer?

	// ensure the finality provider is registered
	if req.FpBtcPk == nil {
		return nil, types.ErrInvalidPubRand.Wrap("empty finality provider public key")
	}
	fpBTCPKBytes := req.FpBtcPk.MustMarshal()

	isBabylonFp := ms.BTCStakingKeeper.BabylonFinalityProviderExists(ctx, fpBTCPKBytes)
	if !isBabylonFp {
		return nil, types.ErrInvalidPubRand.Wrapf("the finality provider with BTC PK %s is not a Babylon Genesis finality provider", req.FpBtcPk.MarshalHex())
	}

	signingContext := signingcontext.FpRandCommitContextV0(ctx.ChainID(), ms.finalityModuleAddress)

	// verify signature over the public randomness commitment
	if err := req.VerifySig(signingContext); err != nil {
		return nil, types.ErrInvalidPubRand.Wrapf("invalid signature over the public randomness list: %v", err)
	}

	prCommit := &types.PubRandCommit{
		StartHeight: req.StartHeight,
		NumPubRand:  req.NumPubRand,
		Commitment:  req.Commitment,
		EpochNum:    ms.GetCurrentEpoch(ctx),
	}

	// get last public randomness commitment
	// TODO: allow committing public randomness earlier than existing ones?
	lastPrCommit := ms.GetLastPubRandCommit(ctx, req.FpBtcPk)

	// this finality provider has not commit any public randomness,
	// commit the given public randomness list and return
	if lastPrCommit == nil {
		if err := ms.SetPubRandCommit(ctx, req.FpBtcPk, prCommit); err != nil {
			return nil, err
		}
		return &types.MsgCommitPubRandListResponse{}, nil
	}

	// ensure height and req.StartHeight do not overlap, i.e., height < req.StartHeight
	lastPrHeightCommitted := lastPrCommit.EndHeight()
	if req.StartHeight <= lastPrCommit.EndHeight() {
		return nil, types.ErrInvalidPubRand.Wrapf("the start height (%d) has overlap with the height of the highest public randomness committed (%d)", req.StartHeight, lastPrHeightCommitted)
	}

	// all good, commit the given public randomness list
	if err := ms.SetPubRandCommit(ctx, req.FpBtcPk, prCommit); err != nil {
		return nil, err
	}
	return &types.MsgCommitPubRandListResponse{}, nil
}

// UnjailFinalityProvider unjails a jailed finality provider
func (ms msgServer) UnjailFinalityProvider(ctx context.Context, req *types.MsgUnjailFinalityProvider) (*types.MsgUnjailFinalityProviderResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyUnjailFinalityProvider)

	// ensure finality provider exists
	fpPk := req.FpBtcPk
	fp, err := ms.BTCStakingKeeper.GetFinalityProvider(ctx, fpPk.MustMarshal())
	if err != nil {
		return nil, fmt.Errorf("failed to get finality provider %s: %w", fpPk.MarshalHex(), err)
	}

	// ensure the signer's address matches the fp's address
	if fp.Addr != req.Signer {
		return nil, fmt.Errorf("the fp's address %s does not match the signer %s of the requestion", fp.Addr, req.Signer)
	}

	// ensure finality provider is already jailed
	if !fp.IsJailed() {
		return nil, bstypes.ErrFpNotJailed
	}

	info, err := ms.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	if err != nil {
		return nil, fmt.Errorf("failed to get the signing info of finality provider %s: %w", fpPk.MarshalHex(), err)
	}

	// cannot be unjailed until jailing period is passed
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	curBlockTime := sdkCtx.HeaderInfo().Time
	jailingPeriodPassed, err := info.IsJailingPeriodPassed(curBlockTime)
	if err != nil {
		return nil, err
	}
	if !jailingPeriodPassed {
		return nil, types.ErrJailingPeriodNotPassed.Wrapf("current block time: %v, required %v", curBlockTime, info.JailedUntil)
	}

	err = ms.BTCStakingKeeper.UnjailFinalityProvider(ctx, fpPk.MustMarshal())
	if err != nil {
		return nil, fmt.Errorf("failed to unjail finality provider %s: %w", fpPk.MarshalHex(), err)
	}

	types.IncrementUnjailedFinalityProviderCounter()

	return &types.MsgUnjailFinalityProviderResponse{}, nil
}

// slashFinalityProvider slashes a finality provider with the given evidence
// including setting its voting power to zero, extracting its BTC SK,
// and emit an event
func (k Keeper) slashFinalityProvider(
	ctx context.Context,
	fpBTCSK *btcec.PrivateKey,
	evidence *types.Evidence,
) error {
	fpBtcPk := bbn.NewBIP340PubKeyFromBTCPK(fpBTCSK.PubKey())

	// slash this finality provider, i.e., set its voting power to zero
	if err := k.BTCStakingKeeper.SlashFinalityProvider(ctx, fpBtcPk.MustMarshal()); err != nil {
		return fmt.Errorf("failed to slash finality provider: %v", err)
	}

	// Propagate slashing information to consumer chains
	if err := k.BTCStakingKeeper.PropagateFPSlashingToConsumers(ctx, fpBTCSK); err != nil {
		return fmt.Errorf("failed to propagate finality provider slashing to consumers: %w", err)
	}

	// emit slashing event
	eventSlashing := types.NewEventSlashedFinalityProvider(evidence)
	if err := sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(eventSlashing); err != nil {
		return fmt.Errorf("failed to emit EventSlashedFinalityProvider event: %w", err)
	}

	return nil
}

// validateActivationHeight returns error if the height received is lower than the finality
// activation block height
func (ms msgServer) validateActivationHeight(ctx sdk.Context, height uint64) (uint64, *errorsmod.Error) {
	// TODO: remove it after Phase-2 launch in a future coordinated upgrade
	activationHeight := ms.GetParams(ctx).FinalityActivationHeight
	if height < activationHeight {
		ms.Logger(ctx).With(
			"height", height,
			"activationHeight", activationHeight,
		).Info("BTC finality is not activated yet")
		return activationHeight, types.ErrFinalityNotActivated
	}
	return activationHeight, nil
}

// EquivocationEvidence handles the evidence of equivocation message sent from the finality gadget cw contract
func (ms msgServer) EquivocationEvidence(goCtx context.Context, req *types.MsgEquivocationEvidence) (*types.MsgEquivocationEvidenceResponse, error) {
	return ms.HandleEquivocationEvidence(goCtx, req)
}
