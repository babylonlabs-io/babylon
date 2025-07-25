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

	ms.SetFinalityProvider(goCtx, fp)

	// notify subscriber
	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := ctx.EventManager().EmitTypedEvent(types.NewEventFinalityProviderEdited(fp)); err != nil {
		panic(fmt.Errorf("failed to emit EventFinalityProviderEdited event: %w", err))
	}

	return &types.MsgEditFinalityProviderResponse{}, nil
}

// CreateBTCDelegation creates a BTC delegation
func (ms msgServer) CreateBTCDelegation(goCtx context.Context, req *types.MsgCreateBTCDelegation) (*types.MsgCreateBTCDelegationResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyCreateBTCDelegation)

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Parses the message into better domain format
	parsedMsg, err := req.ToParsed()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := ms.Keeper.CreateBTCDelegation(ctx, parsedMsg); err != nil {
		return nil, err
	}

	return &types.MsgCreateBTCDelegationResponse{}, nil
}

// BtcStakeExpand creates a BTCDelegation using a previous active staking transaction as one of inputs.
func (ms msgServer) BtcStakeExpand(goCtx context.Context, req *types.MsgBtcStakeExpand) (*types.MsgBtcStakeExpandResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	delInfo, isPreviousStkActive, err := ms.IsBtcDelegationActive(ctx, req.PreviousStakingTxHash)
	if err != nil {
		return nil, err
	}
	prevBtcDel := delInfo.Delegation

	if !isPreviousStkActive {
		return nil, status.Errorf(codes.InvalidArgument, "previous staking transaction is not active")
	}

	if !strings.EqualFold(prevBtcDel.StakerAddr, req.StakerAddr) {
		return nil, status.Errorf(codes.InvalidArgument, "the previous BTC staking transaction staker address: %s does not match with current staker address: %s", prevBtcDel.StakerAddr, req.StakerAddr)
	}

	if !bbn.IsSubsetBip340Pks(prevBtcDel.FpBtcPkList, req.FpBtcPkList) {
		return nil, status.Errorf(codes.InvalidArgument, "the previous BTC staking transaction FPs: %+v are not a subset of the stake expansion FPs %+v", prevBtcDel.FpBtcPkList, req.FpBtcPkList)
	}

	// check FundingTx is not a staking tx
	// ATM is not possible to combine 2 staking txs into one
	fundingTx, err := bbn.NewBTCTxFromBytes(req.FundingTx)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	fundingTxDel := ms.getBTCDelegation(ctx, fundingTx.TxHash())
	if fundingTxDel != nil {
		return nil, status.Error(codes.InvalidArgument, "the funding tx cannot be a staking transaction")
	}

	// Parses the message into better domain format
	parsedMsg, err := req.ToParsed()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	stkExpandTx := parsedMsg.StakingTx.Transaction
	// Check that the input index matches the previous delegation's staking output index
	if prevBtcDel.StakingOutputIdx != stkExpandTx.TxIn[0].PreviousOutPoint.Index {
		return nil, status.Errorf(codes.InvalidArgument, "staking expansion tx input index %d does not match previous delegation staking output index %d",
			stkExpandTx.TxIn[0].PreviousOutPoint.Index, prevBtcDel.StakingOutputIdx)
	}

	// Check that the new delegation staking output amount is >= old delegation staking output amount
	// Assume staking output index is the same as previousBtcDel.StakingOutputIdx
	if int(prevBtcDel.StakingOutputIdx) >= len(stkExpandTx.TxOut) {
		return nil, status.Errorf(codes.InvalidArgument, "staking expansion tx does not have expected output index %d", prevBtcDel.StakingOutputIdx)
	}

	// Validate expansion amount
	newStakingAmt := stkExpandTx.TxOut[prevBtcDel.StakingOutputIdx].Value
	oldStakingAmt := int64(prevBtcDel.TotalSat)
	if err := validateStakeExpansionAmt(parsedMsg, newStakingAmt, oldStakingAmt); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	// Check covenant committee overlap: ensure at least old_quorum covenant members from old params are still active in new params
	oldParams := delInfo.Params
	btcTip := ms.btclcKeeper.GetTipInfo(ctx)
	currentParams, _, err := ms.GetParamsForBtcHeight(ctx, uint64(btcTip.Height))
	if err != nil {
		return nil, err
	}
	if !hasSufficientCovenantOverlap(oldParams.CovenantPks, currentParams.CovenantPks, oldParams.CovenantQuorum) {
		return nil, fmt.Errorf("insufficient covenant committee overlap: need at least %d members from old committee in new committee", oldParams.CovenantQuorum)
	}

	// executes same flow as MsgCreateBTCDelegation pre-approval
	if err := ms.Keeper.CreateBTCDelegation(ctx, parsedMsg); err != nil {
		return nil, err
	}

	return &types.MsgBtcStakeExpandResponse{}, nil
}

// AddBTCDelegationInclusionProof adds inclusion proof of the given delegation on BTC chain
func (ms msgServer) AddBTCDelegationInclusionProof(
	goCtx context.Context,
	req *types.MsgAddBTCDelegationInclusionProof,
) (*types.MsgAddBTCDelegationInclusionProofResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyAddBTCDelegationInclusionProof)

	ctx := sdk.UnwrapSDKContext(goCtx)

	// 1. make sure the delegation exists
	btcDel, err := ms.GetBTCDelegation(ctx, req.StakingTxHash)
	if err != nil {
		return nil, err
	}

	if btcDel.IsStakeExpansion() {
		return nil, fmt.Errorf("the BTC delegation %s is a stake expansion, use MsgBTCUndelegate to set the inclusion proof", req.StakingTxHash)
	}

	// Creates events and updates the btc del if all the checks are valid
	// already has inclusion proof, quorum, wasn't unbonded, if the inclusion proof is valid,
	// k-deep, staking time > unbonding time blocks.
	if err := ms.Keeper.AddBTCDelegationInclusionProof(ctx, btcDel, req.StakingTxInclusionProof); err != nil {
		return nil, err
	}

	// at this point, the BTC delegation inclusion proof is verified and is not duplicated
	// thus, we can safely consider this message as refundable
	ms.ictvKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgAddBTCDelegationInclusionProofResponse{}, nil
}

// AddCovenantSig adds signatures from covenants to a BTC delegation
// TODO: refactor this handler. Now it's too convoluted
func (ms msgServer) AddCovenantSigs(goCtx context.Context, req *types.MsgAddCovenantSigs) (*types.MsgAddCovenantSigsResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeyAddCovenantSigs)

	ctx := sdk.UnwrapSDKContext(goCtx)
	delInfo, err := ms.getBTCDelWithParams(ctx, req.StakingTxHash)
	if err != nil {
		return nil, err
	}
	btcDel, params := delInfo.Delegation, delInfo.Params

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
	status, btcTip, err := ms.BtcDelStatusWithTip(ctx, delInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get BTC delegation status: %w", err)
	}

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

	if err := ms.validateStakeExpansionSig(ctx, delInfo, req); err != nil {
		return nil, types.ErrInvalidCovenantSig.Wrapf("error validating stake expansion signatures: %v", err)
	}

	// All is fine add received signatures to the BTC delegation and BtcUndelegation
	// and emit corresponding events
	ms.addCovenantSigsToBTCDelegation(
		ctx,
		delInfo,
		req.Pk,
		parsedSlashingAdaptorSignatures,
		req.UnbondingTxSig,
		parsedUnbondingSlashingAdaptorSignatures,
		req.StakeExpansionTxSig,
		btcTip.Height,
	)

	// at this point, the covenant signatures are verified and are not duplicated.
	// Thus, we can safely consider this message as refundable
	// NOTE: currently we refund tx fee for covenant signatures even if the BTC
	// delegation already has a covenant quorum. This is to ensure that covenant
	// members do not spend transaction fee, even if they submit covenant signatures
	// late.
	ms.ictvKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgAddCovenantSigsResponse{}, nil
}

// validateStakeExpansionSig validates the covenant signature for a stake expansion.
//
// It performs the following checks:
//   - Ensures that the stake expansion signature is not nil.
//   - Verifies that the signature has not already been received for the given covenant public key.
//   - Validates the signature against the stake expansion transaction.
//
// It returns an error if any of the checks fail.
func (ms msgServer) validateStakeExpansionSig(
	ctx sdk.Context,
	delInfo *btcDelegationWithParams,
	req *types.MsgAddCovenantSigs,
) error {
	if delInfo == nil {
		return fmt.Errorf("nil BTC delegation with params")
	}
	btcDel, params := delInfo.Delegation, delInfo.Params

	if !btcDel.IsStakeExpansion() && req.StakeExpansionTxSig != nil {
		return fmt.Errorf("stake expansion tx signature provided for non-stake expansion delegation")
	}

	if !btcDel.IsStakeExpansion() && req.StakeExpansionTxSig == nil {
		// not stake expansion, no signature provided, ok
		return nil
	}

	if btcDel.IsStakeExpansion() && req.StakeExpansionTxSig == nil {
		return fmt.Errorf("empty stake expansion covenant signature")
	}

	// this is stake expansion delegation, the signature is provided
	// in the message, verify it
	if btcDel.StkExp.IsSignedByCovMember(req.Pk) {
		ms.Logger(ctx).Debug("Received duplicated covenant signature in stake expansion transaction",
			"covenant pk", req.Pk.MarshalHex())
		return errorsmod.Wrapf(types.ErrDuplicatedCovenantSig, "stake expansion transaction")
	}

	// check if the btc pk was a covenant at the parameters version
	// of the previous active staking transaction and signed it
	prevBtcDel, prevParams := delInfo.PrevDel, delInfo.PrevParams

	if !prevParams.HasCovenantPK(req.Pk) {
		return errorsmod.Wrapf(types.ErrInvalidCovenantSig, "covenant with pk %s was not a member at params (version %d) of the previous stake", req.Pk.MarshalHex(), prevBtcDel.ParamsVersion)
	}

	if !prevBtcDel.IsSignedByCovMember(req.Pk) {
		return errorsmod.Wrapf(types.ErrInvalidCovenantSig, "covenant signature for pk %s not found in previous delegation", req.Pk.MarshalHex())
	}

	// Covenant committee members can rotate, so we need to check
	// that there is enough overlap in the covenant committee members
	if !hasSufficientCovenantOverlap(prevParams.CovenantPks, params.CovenantPks, prevParams.CovenantQuorum) {
		return errorsmod.Wrapf(types.ErrInvalidCovenantSig,
			"not enough overlap in covenant committee members for stake expansion: quorum=%d", prevParams.CovenantQuorum)
	}

	otherFundingTxOut, err := btcDel.StkExp.FundingTxOut()
	if err != nil {
		return fmt.Errorf("failed to deserialize other funding txout: %w", err)
	}

	// build staking info of prev delegation
	prevDelStakingInfo, err := prevBtcDel.GetStakingInfo(prevParams, ms.btcNet)
	if err != nil {
		return fmt.Errorf("failed to get staking info of previous delegation: %w", err)
	}
	prevDelUnbondingPathSpendInfo, err := prevDelStakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		return fmt.Errorf("failed to get unbonding path spend info: %w", err)
	}

	err = btcstaking.VerifyTransactionSigStkExp(
		btcDel.MustGetStakingTx(), // this is the staking expansion tx
		prevBtcDel.MustGetStakingTx().TxOut[prevBtcDel.StakingOutputIdx],
		otherFundingTxOut,
		prevDelUnbondingPathSpendInfo.GetPkScriptPath(),
		req.Pk.MustToBTCPK(),
		*req.StakeExpansionTxSig,
	)
	if err != nil {
		return errorsmod.Wrapf(types.ErrInvalidCovenantSig, "bad covenant signature of stake expansion: %v", err)
	}

	return nil
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

	// 1. Check previous delegation exists and is active
	delInfo, err := ms.getBTCDelWithParams(ctx, req.StakingTxHash)
	if err != nil {
		return nil, err
	}

	btcDelStatus, _, err := ms.BtcDelStatusWithTip(ctx, delInfo)
	if err != nil {
		return nil, err
	}
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
	spendStakeTxHash := stakeSpendingTx.TxHash()

	stakeExpansionDel := ms.getBTCDelegation(ctx, spendStakeTxHash)
	isStakeExpansion := stakeExpansionDel != nil && stakeExpansionDel.IsStakeExpansion()

	// 2. Verify stake spending tx inclusion proof
	if isStakeExpansion {
		// Add inclusion proof for stake expansion delegation if stake expansion tx is 'k' deep
		// If successful, this will set stake expansion delegation as active
		if err := ms.Keeper.AddBTCDelegationInclusionProof(ctx, stakeExpansionDel, req.StakeSpendingTxInclusionProof); err != nil {
			return nil, types.ErrInvalidBTCUndelegateReq.Wrapf("failed to handle stake expansion inclusion: %s", err)
		}
	} else {
		proofValid := btcckpttypes.VerifyInclusionProof(
			btcutil.NewTx(stakeSpendingTx),
			&btcHeader.MerkleRoot,
			req.StakeSpendingTxInclusionProof.Proof,
			req.StakeSpendingTxInclusionProof.Key.Index,
		)

		if !proofValid {
			return nil, types.ErrInvalidBTCUndelegateReq.Wrap("stake spending tx is not included in the Bitcoin chain: invalid inclusion proof")
		}
	}

	btcDel := delInfo.Delegation
	stakingTx := btcDel.MustGetStakingTx()
	stakingTxHash := stakingTx.TxHash()

	// 3. Verify stake spending tx spends staking output
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

	// 4. Verify staker signature on stake spending tx
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

	slashingTx := btcDel.MustGetStakingSlashingTx()
	slashingTxHash := slashingTx.TxHash()

	var delegatorUnbondingInfo *types.DelegatorUnbondingInfo

	switch {
	case spendStakeTxHash.IsEqual(&registeredUnbondingTxHash) || spendStakeTxHash.IsEqual(&slashingTxHash):
		delegatorUnbondingInfo = &types.DelegatorUnbondingInfo{
			// if the stake spending tx is the same as the either the registered unbonding tx or the slashing tx,
			// we do not need to save it in the database
			// this is an expected report
			SpendStakeTx: []byte{},
		}
		types.EmitEarlyUnbondedEvent(ctx, btcDel.MustGetStakingTxHash().String(), stakerSpendigTxHeader.Height)
	case isStakeExpansion:
		// stake expansion case: emit stake expansion activation event
		delegatorUnbondingInfo = &types.DelegatorUnbondingInfo{
			SpendStakeTx: req.StakeSpendingTx,
		}
		types.EmitStakeExpansionActivatedEvent(ctx,
			btcDel.MustGetStakingTxHash().String(),
			spendStakeTxHash.String(),
			req.StakeSpendingTxInclusionProof.Key.Hash.MarshalHex(),
			req.StakeSpendingTxInclusionProof.Key.Index,
		)
	default:
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
	ms.ictvKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgBTCUndelegateResponse{}, nil
}

// SelectiveSlashingEvidence handles the evidence that a finality provider has
// selectively slashed a BTC delegation
func (ms msgServer) SelectiveSlashingEvidence(goCtx context.Context, req *types.MsgSelectiveSlashingEvidence) (*types.MsgSelectiveSlashingEvidenceResponse, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), types.MetricsKeySelectiveSlashingEvidence)

	ctx := sdk.UnwrapSDKContext(goCtx)

	// decode the finality provider's BTC SK/PK
	fpSK, fpPK := btcec.PrivKeyFromBytes(req.RecoveredFpBtcSk)
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

	// slashing the provider - this method also checks:
	// - that the fp first exists and can be found
	// - that the finality provider isnt already slashed
	if err := ms.SlashFinalityProvider(ctx, fpBTCPK.MustMarshal()); err != nil {
		return nil, err
	}

	// emit selective slashing event
	evidence := &types.SelectiveSlashingEvidence{
		FpBtcPk:          fpBTCPK,
		RecoveredFpBtcSk: fpSK.Serialize(),
	}
	event := &types.EventSelectiveSlashing{Evidence: evidence}
	if err := sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(event); err != nil {
		panic(fmt.Errorf("failed to emit EventSelectiveSlashing event: %w", err))
	}

	// At this point, the selective slashing evidence is verified and is not duplicated.
	// Thus, we can safely consider this message as refundable
	ms.ictvKeeper.IndexRefundableMsg(ctx, req)

	return &types.MsgSelectiveSlashingEvidenceResponse{}, nil
}

// AddBsnRewards adds rewards for finality providers of a specific BSN consumer
func (ms msgServer) AddBsnRewards(goCtx context.Context, req *types.MsgAddBsnRewards) (*types.MsgAddBsnRewardsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 1. Parse and validate sender address
	senderAddr, err := sdk.AccAddressFromBech32(req.Sender)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address %s: %v", req.Sender, err)
	}

	// 2. Send the rewards to be distributed to that BSN FPs
	err = ms.Keeper.AddBsnRewards(ctx, senderAddr, req.BsnConsumerId, req.TotalRewards, req.FpRatios)
	if err != nil {
		return nil, err
	}

	return &types.MsgAddBsnRewardsResponse{}, nil
}

// hasSufficientCovenantOverlap returns true if the intersection of CovCommittee1 and CovCommittee2
// contains more or equal members than the required overlap.
func hasSufficientCovenantOverlap(
	covCommittee1,
	covCommittee2 []bbn.BIP340PubKey,
	requiredOverlap uint32,
) bool {
	if requiredOverlap == 0 || len(covCommittee1) == 0 || len(covCommittee2) == 0 {
		return false
	}

	// Build a lookup set for newCommittee for efficient membership checks
	newSet := make(map[string]struct{}, len(covCommittee2))
	for _, pk := range covCommittee2 {
		newSet[pk.MarshalHex()] = struct{}{}
	}

	// Count overlapping keys and exit early when quorum is exceeded
	intersection := 0
	for _, oldPk := range covCommittee1 {
		_, found := newSet[oldPk.MarshalHex()]
		if found {
			intersection++
			if uint32(intersection) >= requiredOverlap {
				return true
			}
		}
	}

	return false
}

// validateStakeExpansionAmt ensures the inputs and outputs balance correctly
func validateStakeExpansionAmt(
	parsedMsg *types.ParsedCreateDelegationMessage,
	newStakingAmt int64,
	oldStakingAmt int64,
) error {
	if parsedMsg.StkExp == nil {
		return fmt.Errorf("missing stake expansion data")
	}

	if newStakingAmt < oldStakingAmt {
		return fmt.Errorf("staking expansion output amount %d is less than previous delegation amount %d", newStakingAmt, oldStakingAmt)
	}

	// Calculate expected input value
	fundingInputValue := parsedMsg.StkExp.OtherFundingOutput.Value
	totalInputValue := oldStakingAmt + fundingInputValue

	// Calculate total output value
	stakeExpandTx := parsedMsg.StakingTx.Transaction
	var totalOutputValue int64
	for _, output := range stakeExpandTx.TxOut {
		totalOutputValue += output.Value
	}

	// Calculate implied fee
	impliedFee := totalInputValue - totalOutputValue
	if impliedFee <= 0 {
		return fmt.Errorf("invalid transaction fee: inputs %d <= outputs %d",
			totalInputValue, totalOutputValue)
	}

	return nil
}
