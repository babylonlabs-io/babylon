package keeper

import (
	"context"
	"fmt"
	"strings"
	"time"

	btcckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

// CreateFinalityProvider creates a finality provider
func (ms msgServer) CreateFinalityProvider(goCtx context.Context, req *types.MsgCreateFinalityProvider) (*types.MsgCreateFinalityProviderResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyCreateFinalityProvider)

	// ensure the finality provider address does not already exist
	ctx := sdk.UnwrapSDKContext(goCtx)
	// basic stateless checks
	if err := req.ValidateBasic(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	fpAddr, err := sdk.AccAddressFromBech32(req.Addr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address %s: %v", req.Addr, err)
	}

	// verify proof of possession
	if err := req.Pop.Verify(fpAddr, req.BtcPk, ms.btcNet); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid proof of possession: %v", err)
	}

	if err := ms.AddFinalityProvider(ctx, req); err != nil {
		return nil, err
	}
	return &types.MsgCreateFinalityProviderResponse{}, nil
}

// EditFinalityProvider edits an existing finality provider
func (ms msgServer) EditFinalityProvider(ctx context.Context, req *types.MsgEditFinalityProvider) (*types.MsgEditFinalityProviderResponse, error) {
	// basic stateless checks
	// NOTE: after this, description is guaranteed to be valid
	if err := req.ValidateBasic(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	// ensure commission rate is
	// - at least the minimum commission rate in parameters, and
	// - at most 1
	if req.Commission.LT(ms.MinCommissionRate(ctx)) {
		return nil, types.ErrCommissionLTMinRate.Wrapf("cannot set finality provider commission to less than minimum rate of %s", ms.MinCommissionRate(ctx))
	}
	if req.Commission.GT(sdkmath.LegacyOneDec()) {
		return nil, types.ErrCommissionGTMaxRate
	}

	// TODO: check to index the finality provider by his address instead of the BTC pk
	// find the finality provider with the given BTC PK
	fp, err := ms.GetFinalityProvider(ctx, req.BtcPk)
	if err != nil {
		return nil, err
	}

	fpAddr, err := sdk.AccAddressFromBech32(req.Addr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address %s: %v", req.Addr, err)
	}

	// ensure the signer corresponds to the finality provider's Babylon address
	if !strings.EqualFold(fpAddr.String(), fp.Addr) {
		return nil, status.Errorf(codes.PermissionDenied, "the signer does not correspond to the finality provider's Babylon address")
	}

	// all good, update the finality provider and set back
	fp.Description = req.Description
	fp.Commission = req.Commission
	ms.setFinalityProvider(ctx, fp)

	return &types.MsgEditFinalityProviderResponse{}, nil
}

// CreateBTCDelegation creates a BTC delegation
func (ms msgServer) CreateBTCDelegation(goCtx context.Context, req *types.MsgCreateBTCDelegation) (*types.MsgCreateBTCDelegationResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyCreateBTCDelegation)

	ctx := sdk.UnwrapSDKContext(goCtx)

	// 1. Parse the message into better domain format
	parsedMsg, err := types.ParseCreateDelegationMessage(req)

	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	// 2. Basic stateless checks
	// - verify proof of possession
	if err := parsedMsg.ParsedPop.Verify(parsedMsg.StakerAddress, parsedMsg.StakerPK.BIP340PubKey, ms.btcNet); err != nil {
		return nil, types.ErrInvalidProofOfPossession.Wrap(err.Error())
	}

	// 3. Check if it is not duplicated staking tx
	stakingTxHash := parsedMsg.StakingTx.Transaction.TxHash()
	delgation := ms.getBTCDelegation(ctx, stakingTxHash)
	if delgation != nil {
		return nil, types.ErrReusedStakingTx.Wrapf("duplicated tx hash: %s", stakingTxHash.String())
	}

	// 4. Check finality providers to which message delegate
	// Ensure all finality providers are known to Babylon, are not slashed
	for _, fpBTCPK := range parsedMsg.FinalityProviderKeys.PublicKeysBbnFormat {
		// get this finality provider
		fp, err := ms.GetFinalityProvider(ctx, fpBTCPK)
		if err != nil {
			return nil, err
		}
		// ensure the finality provider is not slashed
		if fp.IsSlashed() {
			return nil, types.ErrFpAlreadySlashed.Wrapf("finality key: %s", fpBTCPK.MarshalHex())
		}
	}

	// 5. Validate parsed message against parameters
	vp := ms.GetParamsWithVersion(ctx)

	btccParams := ms.btccKeeper.GetParams(ctx)

	paramsValidationResult, err := types.ValidateParsedMessageAgainstTheParams(parsedMsg, &vp.Params, &btccParams, ms.btcNet)

	if err != nil {
		return nil, err
	}

	// 6. Check:
	// - timelock of staking tx
	// - staking tx is k-deep
	// - staking tx inclusion proof
	stakingTxHeader := ms.btclcKeeper.GetHeaderByHash(ctx, parsedMsg.StakingTxProofOfInclusion.HeaderHash)

	if stakingTxHeader == nil {
		return nil, fmt.Errorf("header that includes the staking tx is not found")
	}

	// no need to do more validations to the btc header as it was already
	// validated by the btclightclient module
	btcHeader := stakingTxHeader.Header.ToBlockHeader()

	proofValid := btcckpttypes.VerifyInclusionProof(
		btcutil.NewTx(parsedMsg.StakingTx.Transaction),
		&btcHeader.MerkleRoot,
		parsedMsg.StakingTxProofOfInclusion.Proof,
		parsedMsg.StakingTxProofOfInclusion.Index,
	)

	if !proofValid {
		return nil, types.ErrInvalidStakingTx.Wrapf("not included in the Bitcoin chain")
	}

	startHeight := stakingTxHeader.Height

	endHeight := stakingTxHeader.Height + uint64(parsedMsg.StakingTime)

	btcTip := ms.btclcKeeper.GetTipInfo(ctx)
	stakingTxDepth := btcTip.Height - stakingTxHeader.Height
	if stakingTxDepth < btccParams.BtcConfirmationDepth {
		return nil, types.ErrInvalidStakingTx.Wrapf("not k-deep: k=%d; depth=%d", btccParams.BtcConfirmationDepth, stakingTxDepth)
	}
	// ensure staking tx's timelock has more than w BTC blocks left
	if btcTip.Height+btccParams.CheckpointFinalizationTimeout >= endHeight {
		return nil, types.ErrInvalidStakingTx.Wrapf("staking tx's timelock has no more than w(=%d) blocks left", btccParams.CheckpointFinalizationTimeout)
	}

	// 7.all good, construct BTCDelegation and insert BTC delegation
	// NOTE: the BTC delegation does not have voting power yet. It will
	// have voting power only when it receives a covenant signatures
	newBTCDel := &types.BTCDelegation{
		StakerAddr:       parsedMsg.StakerAddress.String(),
		BtcPk:            parsedMsg.StakerPK.BIP340PubKey,
		Pop:              parsedMsg.ParsedPop,
		FpBtcPkList:      parsedMsg.FinalityProviderKeys.PublicKeysBbnFormat,
		StartHeight:      startHeight,
		EndHeight:        endHeight,
		TotalSat:         uint64(parsedMsg.StakingValue),
		StakingTx:        parsedMsg.StakingTx.TransactionBytes,
		StakingOutputIdx: paramsValidationResult.StakingOutputIdx,
		SlashingTx:       types.NewBtcSlashingTxFromBytes(parsedMsg.StakingSlashingTx.TransactionBytes),
		DelegatorSig:     parsedMsg.StakerStakingSlashingTxSig.BIP340Signature,
		UnbondingTime:    uint32(parsedMsg.UnbondingTime),
		CovenantSigs:     nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
		BtcUndelegation: &types.BTCUndelegation{
			UnbondingTx:              parsedMsg.UnbondingTx.TransactionBytes,
			SlashingTx:               types.NewBtcSlashingTxFromBytes(parsedMsg.UnbondingSlashingTx.TransactionBytes),
			DelegatorSlashingSig:     parsedMsg.StakerUnbondingSlashingSig.BIP340Signature,
			DelegatorUnbondingSig:    nil,
			CovenantSlashingSigs:     nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
			CovenantUnbondingSigList: nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
		},
		ParamsVersion: vp.Version, // version of the params against delegations was validated
	}

	// add this BTC delegation, and emit corresponding events
	if err := ms.AddBTCDelegation(ctx, newBTCDel); err != nil {
		panic(fmt.Errorf("failed to add BTC delegation that has passed verification: %w", err))
	}

	return &types.MsgCreateBTCDelegationResponse{}, nil
}

func (ms msgServer) getBTCDelWithParams(
	ctx context.Context,
	stakingTxHash string) (*types.BTCDelegation, *types.Params, error) {
	btcDel, err := ms.GetBTCDelegation(ctx, stakingTxHash)
	if err != nil {
		return nil, nil, err
	}

	bsParams := ms.GetParamsByVersion(ctx, btcDel.ParamsVersion)
	if bsParams == nil {
		panic("params version in BTC delegation is not found")
	}

	return btcDel, bsParams, nil
}

// AddCovenantSig adds signatures from covenants to a BTC delegation
// TODO: refactor this handler. Now it's too convoluted
func (ms msgServer) AddCovenantSigs(goCtx context.Context, req *types.MsgAddCovenantSigs) (*types.MsgAddCovenantSigsResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyAddCovenantSigs)

	ctx := sdk.UnwrapSDKContext(goCtx)
	// basic stateless checks
	if err := req.ValidateBasic(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	btcDel, params, err := ms.getBTCDelWithParams(ctx, req.StakingTxHash)

	if err != nil {
		return nil, err
	}

	// ensure that the given covenant PK is in the parameter
	if !params.HasCovenantPK(req.Pk) {
		return nil, types.ErrInvalidCovenantPK.Wrapf("covenant pk: %s", req.Pk.MarshalHex())
	}

	if btcDel.IsSignedByCovMember(req.Pk) && btcDel.BtcUndelegation.IsSignedByCovMember(req.Pk) {
		ms.Logger(ctx).Debug("Received duplicated covenant signature", "covenant pk", req.Pk.MarshalHex())
		return &types.MsgAddCovenantSigsResponse{}, nil
	}

	if btcDel.HasCovenantQuorums(params.CovenantQuorum) {
		ms.Logger(ctx).Debug("Received covenant signature after achieving quorum", "covenant pk", req.Pk.MarshalHex())
		return &types.MsgAddCovenantSigsResponse{}, nil
	}

	// ensure BTC delegation is still pending, i.e., not expired
	btcTipHeight := ms.btclcKeeper.GetTipInfo(ctx).Height
	wValue := ms.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	status := btcDel.GetStatus(btcTipHeight, wValue, params.CovenantQuorum)
	if status != types.BTCDelegationStatus_PENDING {
		ms.Logger(ctx).Debug("Received covenant signature after the BTC delegation is already expired", "covenant pk", req.Pk.MarshalHex())
		return &types.MsgAddCovenantSigsResponse{}, nil
	}

	// Check that the number of covenant sigs and number of the
	// finality providers are matched
	if len(req.SlashingTxSigs) != len(btcDel.FpBtcPkList) {
		return nil, types.ErrInvalidCovenantSig.Wrapf(
			"number of covenant signatures: %d, number of finality providers being staked to: %d",
			len(req.SlashingTxSigs), len(btcDel.FpBtcPkList))
	}

	/*
		Verify each covenant adaptor signature over slashing tx
	*/
	stakingInfo, err := btcDel.GetStakingInfo(params, ms.btcNet)
	if err != nil {
		panic(fmt.Errorf("failed to get staking info from a verified delegation: %w", err))
	}
	slashingSpendInfo, err := stakingInfo.SlashingPathSpendInfo()
	if err != nil {
		// our staking info was constructed by using BuildStakingInfo constructor, so if
		// this fails, it is a programming error
		panic(err)
	}
	parsedSlashingAdaptorSignatures, err := btcDel.SlashingTx.ParseEncVerifyAdaptorSignatures(
		stakingInfo.StakingOutput,
		slashingSpendInfo,
		req.Pk,
		btcDel.FpBtcPkList,
		req.SlashingTxSigs,
	)
	if err != nil {
		return nil, types.ErrInvalidCovenantSig.Wrapf("err: %v", err)
	}

	// Check that the number of covenant sigs and number of the
	// finality providers are matched
	if len(req.SlashingUnbondingTxSigs) != len(btcDel.FpBtcPkList) {
		return nil, types.ErrInvalidCovenantSig.Wrapf(
			"number of covenant signatures: %d, number of finality providers being staked to: %d",
			len(req.SlashingUnbondingTxSigs), len(btcDel.FpBtcPkList))
	}

	/*
		Verify Schnorr signature over unbonding tx
	*/
	unbondingMsgTx, err := bbn.NewBTCTxFromBytes(btcDel.BtcUndelegation.UnbondingTx)
	if err != nil {
		panic(fmt.Errorf("failed to parse unbonding tx from existing delegation with hash %s : %v", req.StakingTxHash, err))
	}
	unbondingSpendInfo, err := stakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		// our staking info was constructed by using BuildStakingInfo constructor, so if
		// this fails, it is a programming error
		panic(err)
	}
	if err := btcstaking.VerifyTransactionSigWithOutput(
		unbondingMsgTx,
		stakingInfo.StakingOutput,
		unbondingSpendInfo.GetPkScriptPath(),
		req.Pk.MustToBTCPK(),
		*req.UnbondingTxSig,
	); err != nil {
		return nil, types.ErrInvalidCovenantSig.Wrap(err.Error())
	}

	/*
		verify each adaptor signature on slashing unbonding tx
	*/
	unbondingOutput := unbondingMsgTx.TxOut[0] // unbonding tx always have only one output
	unbondingInfo, err := btcDel.GetUnbondingInfo(params, ms.btcNet)
	if err != nil {
		panic(err)
	}
	unbondingSlashingSpendInfo, err := unbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		// our unbonding info was constructed by using BuildStakingInfo constructor, so if
		// this fails, it is a programming error
		panic(err)
	}
	parsedUnbondingSlashingAdaptorSignatures, err := btcDel.BtcUndelegation.SlashingTx.ParseEncVerifyAdaptorSignatures(
		unbondingOutput,
		unbondingSlashingSpendInfo,
		req.Pk,
		btcDel.FpBtcPkList,
		req.SlashingUnbondingTxSigs,
	)
	if err != nil {
		return nil, types.ErrInvalidCovenantSig.Wrapf("err: %v", err)
	}

	// All is fine add received signatures to the BTC delegation and BtcUndelegation
	// and emit corresponding events
	ms.addCovenantSigsToBTCDelegation(
		ctx,
		btcDel,
		req.Pk,
		parsedSlashingAdaptorSignatures,
		req.UnbondingTxSig,
		parsedUnbondingSlashingAdaptorSignatures,
		params,
	)

	return &types.MsgAddCovenantSigsResponse{}, nil
}

// BTCUndelegate adds a signature on the unbonding tx from the BTC delegator
// this effectively proves that the BTC delegator wants to unbond and Babylon
// will consider its BTC delegation unbonded
func (ms msgServer) BTCUndelegate(goCtx context.Context, req *types.MsgBTCUndelegate) (*types.MsgBTCUndelegateResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyBTCUndelegate)

	ctx := sdk.UnwrapSDKContext(goCtx)
	// basic stateless checks
	if err := req.ValidateBasic(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	btcDel, bsParams, err := ms.getBTCDelWithParams(ctx, req.StakingTxHash)

	if err != nil {
		return nil, err
	}

	// ensure the BTC delegation with the given staking tx hash is active
	btcTip := ms.btclcKeeper.GetTipInfo(ctx)
	wValue := ms.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	if btcDel.GetStatus(btcTip.Height, wValue, bsParams.CovenantQuorum) != types.BTCDelegationStatus_ACTIVE {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrap("cannot unbond an inactive BTC delegation")
	}

	// verify the signature on unbonding tx from delegator
	unbondingMsgTx, err := bbn.NewBTCTxFromBytes(btcDel.BtcUndelegation.UnbondingTx)
	if err != nil {
		panic(fmt.Errorf("failed to parse unbonding tx from existing delegation with hash %s : %v", req.StakingTxHash, err))
	}
	stakingInfo, err := btcDel.GetStakingInfo(bsParams, ms.btcNet)
	if err != nil {
		panic(fmt.Errorf("failed to get staking info from a verified delegation: %w", err))
	}
	unbondingSpendInfo, err := stakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		// our staking info was constructed by using BuildStakingInfo constructor, so if
		// this fails, it is a programming error
		panic(err)
	}
	if err := btcstaking.VerifyTransactionSigWithOutput(
		unbondingMsgTx,
		stakingInfo.StakingOutput,
		unbondingSpendInfo.GetPkScriptPath(),
		btcDel.BtcPk.MustToBTCPK(),
		*req.UnbondingTxSig,
	); err != nil {
		return nil, types.ErrInvalidCovenantSig.Wrap(err.Error())
	}

	// all good, add the signature to BTC delegation's undelegation
	// and set back
	ms.btcUndelegate(ctx, btcDel, req.UnbondingTxSig)

	return &types.MsgBTCUndelegateResponse{}, nil
}

// SelectiveSlashingEvidence handles the evidence that a finality provider has
// selectively slashed a BTC delegation
func (ms msgServer) SelectiveSlashingEvidence(goCtx context.Context, req *types.MsgSelectiveSlashingEvidence) (*types.MsgSelectiveSlashingEvidenceResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeySelectiveSlashingEvidence)

	ctx := sdk.UnwrapSDKContext(goCtx)

	btcDel, bsParams, err := ms.getBTCDelWithParams(ctx, req.StakingTxHash)

	if err != nil {
		return nil, err
	}

	// ensure the BTC delegation is active, or its BTC undelegation receives an
	// unbonding signature from the staker
	btcTip := ms.btclcKeeper.GetTipInfo(ctx)
	wValue := ms.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	covQuorum := bsParams.CovenantQuorum
	if btcDel.GetStatus(btcTip.Height, wValue, covQuorum) != types.BTCDelegationStatus_ACTIVE && !btcDel.IsUnbondedEarly() {
		return nil, types.ErrBTCDelegationNotFound.Wrap("a BTC delegation that is not active or unbonding early cannot be slashed")
	}

	// decode the finality provider's BTC SK/PK
	fpSK, fpPK := btcec.PrivKeyFromBytes(req.RecoveredFpBtcSk)
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

	// ensure the BTC delegation is staked to the given finality provider
	fpIdx := btcDel.GetFpIdx(fpBTCPK)
	if fpIdx == -1 {
		return nil, types.ErrFpNotFound.Wrapf("BTC delegation is not staked to the finality provider")
	}

	// ensure the finality provider exists
	fp, err := ms.GetFinalityProvider(ctx, fpBTCPK.MustMarshal())
	if err != nil {
		panic(types.ErrFpNotFound.Wrapf("failing to find the finality provider with BTC delegations"))
	}
	// ensure the finality provider is not slashed
	if fp.IsSlashed() {
		return nil, types.ErrFpAlreadySlashed
	}

	// at this point, the finality provider must have done selective slashing and must be
	// adversarial

	// slash the finality provider now
	if err := ms.SlashFinalityProvider(ctx, fpBTCPK.MustMarshal()); err != nil {
		panic(err) // failed to slash the finality provider, must be programming error
	}

	// emit selective slashing event
	evidence := &types.SelectiveSlashingEvidence{
		StakingTxHash:    req.StakingTxHash,
		FpBtcPk:          fpBTCPK,
		RecoveredFpBtcSk: fpSK.Serialize(),
	}
	event := &types.EventSelectiveSlashing{Evidence: evidence}
	if err := sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(event); err != nil {
		panic(fmt.Errorf("failed to emit EventSelectiveSlashing event: %w", err))
	}

	return &types.MsgSelectiveSlashingEvidenceResponse{}, nil
}
