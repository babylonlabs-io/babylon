package keeper

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	btcckpttypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"

	errorsmod "cosmossdk.io/errors"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/babylonlabs-io/babylon/v3/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
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
	// req.Params validation is done in ValidateBasic
	if ms.authority != req.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, req.Authority)
	}

	// ensure the min unbonding time is always larger than the checkpoint finalization timeout
	ctx := sdk.UnwrapSDKContext(goCtx)
	ckptFinalizationTime := ms.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	unbondingTime := req.Params.UnbondingTimeBlocks
	if unbondingTime <= ckptFinalizationTime {
		return nil, govtypes.ErrInvalidProposalMsg.
			Wrapf("the unbonding time %d must be larger than the checkpoint finalization timeout %d",
				unbondingTime, ckptFinalizationTime)
	}

	if err := ms.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

// CreateFinalityProvider creates a finality provider
func (ms msgServer) CreateFinalityProvider(goCtx context.Context, req *types.MsgCreateFinalityProvider) (*types.MsgCreateFinalityProviderResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyCreateFinalityProvider)

	// ensure the finality provider address does not already exist
	fpAddr, err := sdk.AccAddressFromBech32(req.Addr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address %s: %v", req.Addr, err)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	signingContext := signingcontext.FpPopContextV0(ctx.ChainID(), ms.btcStakingModuleAddress)

	// verify proof of possession
	if err := req.Pop.Verify(signingContext, fpAddr, req.BtcPk, ms.btcNet); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid proof of possession: %v", err)
	}

	if err := ms.AddFinalityProvider(ctx, req); err != nil {
		return nil, err
	}
	return &types.MsgCreateFinalityProviderResponse{}, nil
}

// EditFinalityProvider edits an existing finality provider
func (ms msgServer) EditFinalityProvider(goCtx context.Context, req *types.MsgEditFinalityProvider) (*types.MsgEditFinalityProviderResponse, error) {
	fpAddr, err := sdk.AccAddressFromBech32(req.Addr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address %s: %v", req.Addr, err)
	}

	fp, err := ms.GetFinalityProvider(goCtx, req.BtcPk)
	if err != nil {
		return nil, err
	}

	// ensure the signer corresponds to the finality provider's Babylon address
	if !strings.EqualFold(fpAddr.String(), fp.Addr) {
		return nil, status.Errorf(codes.PermissionDenied, "the signer does not correspond to the finality provider's Babylon address")
	}

	if err := ms.UpdateFinalityProviderCommission(goCtx, req.Commission, fp); err != nil {
		return nil, err
	}

	// all good, update the finality provider and set back
	fp.Description = req.Description

	ms.setFinalityProvider(goCtx, fp)

	// notify subscriber
	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := ctx.EventManager().EmitTypedEvent(types.NewEventFinalityProviderEdited(fp)); err != nil {
		panic(fmt.Errorf("failed to emit EventFinalityProviderEdited event: %w", err))
	}

	return &types.MsgEditFinalityProviderResponse{}, nil
}

// isAllowListEnabled checks if the allow list is enabled at the given height
// allow list is enabled if AllowListExpirationHeight is larger than 0,
// and current block height is less than AllowListExpirationHeight
func (ms msgServer) isAllowListEnabled(ctx sdk.Context, p *types.Params) bool {
	return p.AllowListExpirationHeight > 0 && uint64(ctx.BlockHeight()) < p.AllowListExpirationHeight
}

func (ms msgServer) getTimeInfoAndParams(
	ctx sdk.Context,
	parsedMsg *types.ParsedCreateDelegationMessage,
) (*DelegationTimeRangeInfo, *types.Params, uint32, error) {
	if parsedMsg.IsIncludedOnBTC() {
		// staking tx is already included on BTC
		// 1. Validate inclusion proof and retrieve inclusion height
		// 2. Get params for the validated inclusion height
		btccParams := ms.btccKeeper.GetParams(ctx)

		timeInfo, err := ms.VerifyInclusionProofAndGetHeight(
			ctx,
			btcutil.NewTx(parsedMsg.StakingTx.Transaction),
			btccParams.BtcConfirmationDepth,
			uint32(parsedMsg.StakingTime),
			uint32(parsedMsg.UnbondingTime),
			parsedMsg.StakingTxProofOfInclusion,
		)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("invalid inclusion proof: %w", err)
		}

		paramsByHeight, version, err := ms.GetParamsForBtcHeight(ctx, uint64(timeInfo.StartHeight))
		if err != nil {
			// this error can happen if we receive delegations which is included before
			// first activation height we support
			return nil, nil, 0, err
		}

		return timeInfo, paramsByHeight, version, nil
	}
	// staking tx is not included on BTC, retrieve params for the current tip height
	// and return info about the tip
	btcTip := ms.btclcKeeper.GetTipInfo(ctx)

	paramsByHeight, version, err := ms.GetParamsForBtcHeight(ctx, uint64(btcTip.Height))
	if err != nil {
		return nil, nil, 0, err
	}

	return &DelegationTimeRangeInfo{
		StartHeight: 0,
		EndHeight:   0,
		TipHeight:   btcTip.Height,
	}, paramsByHeight, version, nil
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

	signingContext := signingcontext.StakerPopContextV0(ctx.ChainID(), ms.btcStakingModuleAddress)

	// 2. Basic stateless checks
	// - verify proof of possession
	if err := parsedMsg.ParsedPop.Verify(signingContext, parsedMsg.StakerAddress, parsedMsg.StakerPK.BIP340PubKey, ms.btcNet); err != nil {
		return nil, types.ErrInvalidProofOfPossession.Wrap(err.Error())
	}

	// 3. Check if it is not duplicated staking tx
	stakingTxHash := parsedMsg.StakingTx.Transaction.TxHash()
	delegation := ms.getBTCDelegation(ctx, stakingTxHash)
	if delegation != nil {
		return nil, types.ErrReusedStakingTx.Wrapf("duplicated tx hash: %s", stakingTxHash.String())
	}

	// Ensure all finality providers
	// - are known to Babylon,
	// - at least 1 one of them is a Babylon finality provider,
	// - are not slashed, and
	// - their registered epochs are finalised
	// and then check whether the BTC stake is multi-staked to FPs of consumers
	// TODO: ensure the BTC delegation does not multi-stake to too many finality providers
	multiStakedToConsumers, err := ms.validateMultiStakedFPs(ctx, parsedMsg.FinalityProviderKeys.PublicKeysBbnFormat)
	if err != nil {
		return nil, err
	}

	// 5. Get params for the validated inclusion height either tip or inclusion height
	timeInfo, params, paramsVersion, err := ms.getTimeInfoAndParams(ctx, parsedMsg)
	if err != nil {
		return nil, err
	}

	// 6. Validate the staking tx against the params
	paramsValidationResult, err := types.ValidateParsedMessageAgainstTheParams(parsedMsg, params, ms.btcNet)

	if err != nil {
		return nil, err
	}

	// 7. if allow list is enabled we need to check whether staking transactions hash
	// is in the allow list
	if ms.isAllowListEnabled(ctx, params) {
		if !ms.IsStakingTransactionAllowed(ctx, &stakingTxHash) {
			return nil, types.ErrInvalidStakingTx.Wrapf("staking tx hash: %s, is not in the allow list", stakingTxHash.String())
		}
	}

	// everything is good, if the staking tx is not included on BTC consume additinal
	// gas
	if !parsedMsg.IsIncludedOnBTC() {
		ctx.GasMeter().ConsumeGas(params.DelegationCreationBaseGasFee, "delegation creation fee")
	}

	// 7.all good, construct BTCDelegation and insert BTC delegation
	// NOTE: the BTC delegation does not have voting power yet. It will
	// have voting power only when it receives a covenant signatures
	newBTCDel := &types.BTCDelegation{
		StakerAddr:       parsedMsg.StakerAddress.String(),
		BtcPk:            parsedMsg.StakerPK.BIP340PubKey,
		Pop:              parsedMsg.ParsedPop,
		FpBtcPkList:      parsedMsg.FinalityProviderKeys.PublicKeysBbnFormat,
		StakingTime:      uint32(parsedMsg.StakingTime),
		StartHeight:      timeInfo.StartHeight,
		EndHeight:        timeInfo.EndHeight,
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
			CovenantSlashingSigs:     nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
			CovenantUnbondingSigList: nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
			DelegatorUnbondingInfo:   nil,
		},
		ParamsVersion: paramsVersion,      // version of the params against which delegation was validated
		BtcTipHeight:  timeInfo.TipHeight, // height of the BTC light client tip at the time of the delegation creation
	}

	// add this BTC delegation, and emit corresponding events
	if err := ms.AddBTCDelegation(ctx, newBTCDel); err != nil {
		panic(fmt.Errorf("failed to add BTC delegation that has passed verification: %w", err))
	}
	// if this BTC delegation is multi-staked to consumers' FPs, add it to btcstkconsumer indexes
	// TODO: revisit the relationship between BTC staking module and BTC staking consumer module
	if multiStakedToConsumers {
		if err := ms.indexBTCConsumerDelegation(ctx, newBTCDel); err != nil {
			panic(fmt.Errorf("failed to add BTC delegation multi-staked to consumers' finality providers despite it has passed verification: %w", err))
		}
	}

	return &types.MsgCreateBTCDelegationResponse{}, nil
}

// AddBTCDelegationInclusionProof adds inclusion proof of the given delegation on BTC chain
func (ms msgServer) AddBTCDelegationInclusionProof(
	goCtx context.Context,
	req *types.MsgAddBTCDelegationInclusionProof,
) (*types.MsgAddBTCDelegationInclusionProofResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyAddBTCDelegationInclusionProof)

	ctx := sdk.UnwrapSDKContext(goCtx)

	// 1. make sure the delegation exists
	btcDel, params, err := ms.getBTCDelWithParams(ctx, req.StakingTxHash)
	if err != nil {
		return nil, err
	}

	// 2. check if the delegation already has inclusion proof
	if btcDel.HasInclusionProof() {
		return nil, fmt.Errorf("the delegation %s already has inclusion proof", req.StakingTxHash)
	}

	// 3. check if the delegation has received a quorum of covenant sigs
	if !btcDel.HasCovenantQuorums(params.CovenantQuorum) {
		return nil, fmt.Errorf("the delegation %s has not received a quorum of covenant signatures", req.StakingTxHash)
	}

	// 4. check if the delegation is already unbonded
	if btcDel.BtcUndelegation.DelegatorUnbondingInfo != nil {
		return nil, fmt.Errorf("the delegation %s is already unbonded", req.StakingTxHash)
	}

	// 5. verify inclusion proof
	parsedInclusionProof, err := types.NewParsedProofOfInclusion(req.StakingTxInclusionProof)
	if err != nil {
		return nil, err
	}
	stakingTx, err := bbn.NewBTCTxFromBytes(btcDel.StakingTx)
	if err != nil {
		return nil, err
	}

	btccParams := ms.btccKeeper.GetParams(ctx)

	timeInfo, err := ms.VerifyInclusionProofAndGetHeight(
		ctx,
		btcutil.NewTx(stakingTx),
		btccParams.BtcConfirmationDepth,
		btcDel.StakingTime,
		params.UnbondingTimeBlocks,
		parsedInclusionProof,
	)

	if err != nil {
		return nil, fmt.Errorf("invalid inclusion proof: %w", err)
	}

	// 6. check if the staking tx is included after the BTC tip height at the time of the delegation creation
	if timeInfo.StartHeight < btcDel.BtcTipHeight {
		return nil, types.ErrStakingTxIncludedTooEarly.Wrapf(
			"btc tip height at the time of the delegation creation: %d, staking tx inclusion height: %d",
			btcDel.BtcTipHeight,
			timeInfo.StartHeight,
		)
	}

	// 7. set start height and end height and save it to db
	btcDel.StartHeight = timeInfo.StartHeight
	btcDel.EndHeight = timeInfo.EndHeight
	ms.setBTCDelegation(ctx, btcDel)

	// 8. emit events
	stakingTxHash := btcDel.MustGetStakingTxHash()

	newInclusionProofEvent := types.NewInclusionProofEvent(
		stakingTxHash.String(),
		btcDel.StartHeight,
		btcDel.EndHeight,
		types.BTCDelegationStatus_ACTIVE,
	)

	if err := ctx.EventManager().EmitTypedEvents(newInclusionProofEvent); err != nil {
		panic(fmt.Errorf("failed to emit events for the new active BTC delegation: %w", err))
	}

	activeEvent := types.NewEventPowerDistUpdateWithBTCDel(
		&types.EventBTCDelegationStateUpdate{
			StakingTxHash: stakingTxHash.String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		},
	)

	// notify consumer chains about the active BTC delegation
	ms.notifyConsumersOnActiveBTCDel(ctx, btcDel)

	ms.addPowerDistUpdateEvent(ctx, timeInfo.TipHeight, activeEvent)

	// record event that the BTC delegation will become unbonded at EndHeight-w
	expiredEvent := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
		StakingTxHash: req.StakingTxHash,
		NewState:      types.BTCDelegationStatus_EXPIRED,
	})

	// NOTE: we should have verified that EndHeight > btcTip.Height + min_unbonding_time
	ms.addPowerDistUpdateEvent(ctx, btcDel.EndHeight-params.UnbondingTimeBlocks, expiredEvent)

	// at this point, the BTC delegation inclusion proof is verified and is not duplicated
	// thus, we can safely consider this message as refundable
	ms.iKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgAddBTCDelegationInclusionProofResponse{}, nil
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
		// return error if the covenant signature is already submitted
		// this is to secure the tx refunding against duplicated messages
		return nil, types.ErrDuplicatedCovenantSig
	}

	// ensure BTC delegation is still pending, i.e., not unbonded
	btcTipHeight := ms.btclcKeeper.GetTipInfo(ctx).Height
	status := btcDel.GetStatus(btcTipHeight, params.CovenantQuorum)
	if status == types.BTCDelegationStatus_UNBONDED || status == types.BTCDelegationStatus_EXPIRED {
		ms.Logger(ctx).Debug("Received covenant signature after the BTC delegation is already unbonded", "covenant pk", req.Pk.MarshalHex())
		return nil, types.ErrInvalidCovenantSig.Wrap("the BTC delegation is already unbonded")
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
		btcTipHeight,
	)

	// at this point, the covenant signatures are verified and are not duplicated.
	// Thus, we can safely consider this message as refundable
	// NOTE: currently we refund tx fee for covenant signatures even if the BTC
	// delegation already has a covenant quorum. This is to ensure that covenant
	// members do not spend transaction fee, even if they submit covenant signatures
	// late.
	ms.iKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgAddCovenantSigsResponse{}, nil
}

func findInputIdx(
	tx *wire.MsgTx,
	fundingTxHash *chainhash.Hash,
	fundingOutputIdx uint32,
) (uint32, error) {
	for idx, txIn := range tx.TxIn {
		if txIn.PreviousOutPoint.Hash.IsEqual(fundingTxHash) && txIn.PreviousOutPoint.Index == fundingOutputIdx {
			return uint32(idx), nil
		}
	}
	return 0, fmt.Errorf("transaction does not spend the expected output %s:%d", fundingTxHash.String(), fundingOutputIdx)
}

func getFundingTxTransactions(txs [][]byte) ([]*wire.MsgTx, error) {
	if len(txs) == 0 {
		return nil, fmt.Errorf("no funding transactions provided")
	}

	fundingTxs := make([]*wire.MsgTx, len(txs))

	for i, tx := range txs {
		fundingTx, err := bbn.NewBTCTxFromBytes(tx)

		if err != nil {
			return nil, fmt.Errorf("failed to parse funding transaction: %w", err)
		}

		fundingTxs[i] = fundingTx
	}

	return fundingTxs, nil
}

// BTCUndelegate adds a signature on the unbonding tx from the BTC delegator
// this effectively proves that the BTC delegator wants to unbond and Babylon
// will consider its BTC delegation unbonded
func (ms msgServer) BTCUndelegate(goCtx context.Context, req *types.MsgBTCUndelegate) (*types.MsgBTCUndelegateResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyBTCUndelegate)

	ctx := sdk.UnwrapSDKContext(goCtx)
	btcDel, bsParams, err := ms.getBTCDelWithParams(ctx, req.StakingTxHash)

	if err != nil {
		return nil, err
	}

	// ensure the BTC delegation with the given staking tx hash is active
	btcTip := ms.btclcKeeper.GetTipInfo(ctx)

	btcDelStatus := btcDel.GetStatus(
		btcTip.Height,
		bsParams.CovenantQuorum,
	)

	if btcDelStatus == types.BTCDelegationStatus_UNBONDED || btcDelStatus == types.BTCDelegationStatus_EXPIRED {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrap("cannot unbond an unbonded BTC delegation")
	}

	stakeSpendingTx, err := bbn.NewBTCTxFromBytes(req.StakeSpendingTx)

	if err != nil {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrapf("failed to parse staking spending tx: %v", err)
	}

	stakerSpendigTxHeader, err := ms.btclcKeeper.GetHeaderByHash(ctx, req.StakeSpendingTxInclusionProof.Key.Hash)
	if err != nil {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrapf("stake spending tx is not on BTC chain: %v", err)
	}

	btcHeader := stakerSpendigTxHeader.Header.ToBlockHeader()

	// 1. Verify stake spending tx inclusion proof
	proofValid := btcckpttypes.VerifyInclusionProof(
		btcutil.NewTx(stakeSpendingTx),
		&btcHeader.MerkleRoot,
		req.StakeSpendingTxInclusionProof.Proof,
		req.StakeSpendingTxInclusionProof.Key.Index,
	)

	if !proofValid {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrap("stake spending tx is not included in the Bitcoin chain: invalid inclusion proof")
	}

	stakingTx := btcDel.MustGetStakingTx()

	stakingTxHash := stakingTx.TxHash()

	// 2. Verify stake spending tx spends staking output
	stakingTxInputIdx, err := findInputIdx(
		stakeSpendingTx,
		&stakingTxHash,
		btcDel.StakingOutputIdx,
	)

	if err != nil {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrapf("stake spending tx does not spend staking output: %s", err)
	}

	fundingTxs, err := getFundingTxTransactions(req.FundingTransactions)

	if err != nil {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrapf("failed to parse funding transactions: %s", err)
	}

	// 3. Verify staker signature on stake spending tx
	if err := VerifySpendStakeTxStakerSig(
		btcDel.BtcPk.MustToBTCPK(),
		stakingTx.TxOut[btcDel.StakingOutputIdx],
		stakingTxInputIdx,
		fundingTxs,
		stakeSpendingTx,
	); err != nil {
		return nil, types.ErrInvalidBTCUndelegateReq.Wrapf("failed to verify stake spending tx staker signature: %s", err)
	}

	registeredUnbondingTx := btcDel.MustGetUnbondingTx()

	registeredUnbondingTxHash := registeredUnbondingTx.TxHash()

	spendStakeTxHash := stakeSpendingTx.TxHash()

	var delegatorUnbondingInfo *types.DelegatorUnbondingInfo

	// Check if stake spending tx is already registered unbonding tx. If so, we do
	// not need to save it in database
	if spendStakeTxHash.IsEqual(&registeredUnbondingTxHash) {
		delegatorUnbondingInfo = &types.DelegatorUnbondingInfo{
			// if the stake spending tx is the same as the registered unbonding tx,
			// we do not need to save it in the database
			SpendStakeTx: []byte{},
		}

		types.EmitEarlyUnbondedEvent(ctx, btcDel.MustGetStakingTxHash().String(), stakerSpendigTxHeader.Height)
	} else {
		// spend staking tx is not the registered unbonding tx, we need to save it in the database
		// and emit an event
		delegatorUnbondingInfo = &types.DelegatorUnbondingInfo{
			SpendStakeTx: req.StakeSpendingTx,
		}

		types.EmitUnexpectedUnbondingTxEvent(ctx,
			btcDel.MustGetStakingTxHash().String(),
			spendStakeTxHash.String(),
			req.StakeSpendingTxInclusionProof.Key.Hash.MarshalHex(),
			req.StakeSpendingTxInclusionProof.Key.Index,
		)
	}

	// all good, add the signature to BTC delegation's undelegation
	// and set back
	ms.btcUndelegate(ctx, btcDel, delegatorUnbondingInfo, req.StakeSpendingTx, req.StakeSpendingTxInclusionProof)

	// At this point, the unbonding signature is verified.
	// Thus, we can safely consider this message as refundable
	ms.iKeeper.IndexRefundableMsg(ctx, req)

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
	covQuorum := bsParams.CovenantQuorum
	if btcDel.GetStatus(btcTip.Height, covQuorum) != types.BTCDelegationStatus_ACTIVE && !btcDel.IsUnbondedEarly() {
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

	// At this point, the selective slashing evidence is verified and is not duplicated.
	// Thus, we can safely consider this message as refundable
	ms.iKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgSelectiveSlashingEvidenceResponse{}, nil
}
