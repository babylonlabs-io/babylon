package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

type msgServer struct {
	Keeper
}

var _ types.MsgServer = msgServer{}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// WrappedEditValidator handles the MsgWrappedEditValidator request
func (ms msgServer) WrappedEditValidator(goCtx context.Context, msgWrapped *types.MsgWrappedEditValidator) (*types.MsgWrappedEditValidatorResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msgWrapped.Msg == nil {
		return nil, types.ErrNoWrappedMsg
	}
	msg := msgWrapped.Msg

	// verification rules ported from staking module
	valAddr, valErr := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if valErr != nil {
		return nil, valErr
	}

	if msg.Description == (stktypes.Description{}) {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "empty description")
	}

	if msg.MinSelfDelegation != nil && !msg.MinSelfDelegation.IsPositive() {
		return nil, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"minimum self delegation must be a positive integer",
		)
	}

	if msg.CommissionRate != nil {
		if msg.CommissionRate.GT(math.LegacyOneDec()) || msg.CommissionRate.IsNegative() {
			return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "commission rate must be between 0 and 1 (inclusive)")
		}

		stkParams, err := ms.stk.GetParams(ctx)
		if err != nil {
			return nil, errorsmod.Wrap(sdkerrors.ErrLogic, err.Error())
		}

		minCommissionRate := stkParams.MinCommissionRate

		if msg.CommissionRate.LT(minCommissionRate) {
			return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "commission rate cannot be less than the min commission rate %s", minCommissionRate.String())
		}
	}

	// validator must already be registered
	_, err := ms.stk.GetValidator(ctx, valAddr)
	if err != nil {
		return nil, err
	}

	blockHeight := uint64(ctx.HeaderInfo().Height)
	if blockHeight == 0 {
		return nil, types.ErrZeroEpochMsg
	}
	blockTime := ctx.HeaderInfo().Time

	txid := tmhash.Sum(ctx.TxBytes())
	queuedMsg, err := types.NewQueuedMessage(blockHeight, blockTime, txid, msg)
	if err != nil {
		return nil, err
	}

	ms.EnqueueMsg(ctx, queuedMsg)

	ctx.GasMeter().ConsumeGas(ms.GetParams(ctx).EnqueueGasFees.EditValidator, "epoching staking update params enqueue fee")

	err = ctx.EventManager().EmitTypedEvents(
		&types.EventWrappedEditValidator{
			ValidatorAddress: msg.ValidatorAddress,
			EpochBoundary:    ms.GetEpoch(ctx).GetLastBlockHeight(),
		},
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgWrappedEditValidatorResponse{}, nil
}

// WrappedStakingUpdateParams handles the MsgWrappedStakingUpdateParams request
func (ms msgServer) WrappedStakingUpdateParams(goCtx context.Context, msgWrapped *types.MsgWrappedStakingUpdateParams) (*types.MsgWrappedStakingUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msgWrapped.Msg == nil {
		return nil, types.ErrNoWrappedMsg
	}
	msg := msgWrapped.Msg
	if ms.authority != msg.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, msg.Authority)
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}

	blockHeight := uint64(ctx.HeaderInfo().Height)
	if blockHeight == 0 {
		return nil, types.ErrZeroEpochMsg
	}
	blockTime := ctx.HeaderInfo().Time

	txid := tmhash.Sum(ctx.TxBytes())
	queuedMsg, err := types.NewQueuedMessage(blockHeight, blockTime, txid, msg)
	if err != nil {
		return nil, err
	}

	ms.EnqueueMsg(ctx, queuedMsg)

	ctx.GasMeter().ConsumeGas(ms.GetParams(ctx).EnqueueGasFees.StakingUpdateParams, "epoching staking update params enqueue fee")

	err = ctx.EventManager().EmitTypedEvents(
		&types.EventWrappedStakingUpdateParams{
			UnbondingTime:     msg.Params.UnbondingTime.String(),
			MaxValidators:     msg.Params.MaxValidators,
			MaxEntries:        msg.Params.MaxEntries,
			HistoricalEntries: msg.Params.HistoricalEntries,
			BondDenom:         msg.Params.BondDenom,
			MinCommissionRate: msg.Params.MinCommissionRate.String(),
			EpochBoundary:     ms.GetEpoch(ctx).GetLastBlockHeight(),
		},
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgWrappedStakingUpdateParamsResponse{}, nil
}

// WrappedDelegate handles the MsgWrappedDelegate request
func (ms msgServer) WrappedDelegate(goCtx context.Context, msg *types.MsgWrappedDelegate) (*types.MsgWrappedDelegateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Msg == nil {
		return nil, types.ErrNoWrappedMsg
	}

	// verification rules ported from staking module
	valAddr, valErr := sdk.ValAddressFromBech32(msg.Msg.ValidatorAddress)
	if valErr != nil {
		return nil, valErr
	}
	if _, err := ms.stk.GetValidator(ctx, valAddr); err != nil {
		return nil, err
	}
	if _, err := sdk.AccAddressFromBech32(msg.Msg.DelegatorAddress); err != nil {
		return nil, err
	}
	bondDenom, err := ms.stk.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	if msg.Msg.Amount.Denom != bondDenom {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "invalid coin denomination: got %s, expected %s", msg.Msg.Amount.Denom, bondDenom,
		)
	}

	params := ms.GetParams(ctx)
	if msg.Msg.Amount.Amount.LT(math.NewIntFromUint64(params.MinAmount)) {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"delegation amount %s is below minimum required amount %d",
			msg.Msg.Amount.Amount.String(),
			params.MinAmount,
		)
	}

	blockHeight := uint64(ctx.HeaderInfo().Height)
	if blockHeight == 0 {
		return nil, types.ErrZeroEpochMsg
	}
	blockTime := ctx.HeaderInfo().Time

	txid := tmhash.Sum(ctx.TxBytes())
	queuedMsg, err := types.NewQueuedMessage(blockHeight, blockTime, txid, msg)
	if err != nil {
		return nil, err
	}

	if err := ms.LockFunds(ctx, &queuedMsg); err != nil {
		return nil, errorsmod.Wrap(err, "failed to lock user funds")
	}
	ms.EnqueueMsg(ctx, queuedMsg)

	ctx.GasMeter().ConsumeGas(ms.GetParams(ctx).EnqueueGasFees.Delegate, "epoching delegate enqueue fee")

	err = ctx.EventManager().EmitTypedEvents(
		&types.EventWrappedDelegate{
			DelegatorAddress: msg.Msg.DelegatorAddress,
			ValidatorAddress: msg.Msg.ValidatorAddress,
			Amount:           msg.Msg.Amount.Amount.Uint64(),
			Denom:            msg.Msg.Amount.GetDenom(),
			EpochBoundary:    ms.GetEpoch(ctx).GetLastBlockHeight(),
		},
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgWrappedDelegateResponse{}, nil
}

// WrappedUndelegate handles the MsgWrappedUndelegate request
func (ms msgServer) WrappedUndelegate(goCtx context.Context, msg *types.MsgWrappedUndelegate) (*types.MsgWrappedUndelegateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Msg == nil {
		return nil, types.ErrNoWrappedMsg
	}

	// verification rules ported from staking module
	valAddr, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}
	delegatorAddress, err := sdk.AccAddressFromBech32(msg.Msg.DelegatorAddress)
	if err != nil {
		return nil, err
	}
	if _, err := ms.stk.ValidateUnbondAmount(ctx, delegatorAddress, valAddr, msg.Msg.Amount.Amount); err != nil {
		return nil, err
	}
	bondDenom, err := ms.stk.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	if msg.Msg.Amount.Denom != bondDenom {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "invalid coin denomination: got %s, expected %s", msg.Msg.Amount.Denom, bondDenom,
		)
	}

	params := ms.GetParams(ctx)
	if msg.Msg.Amount.Amount.LT(math.NewIntFromUint64(params.MinAmount)) {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"undelegation amount %s is below minimum required amount %d",
			msg.Msg.Amount.Amount.String(),
			params.MinAmount,
		)
	}

	blockHeight := uint64(ctx.HeaderInfo().Height)
	if blockHeight == 0 {
		return nil, types.ErrZeroEpochMsg
	}
	blockTime := ctx.HeaderInfo().Time

	txid := tmhash.Sum(ctx.TxBytes())
	queuedMsg, err := types.NewQueuedMessage(blockHeight, blockTime, txid, msg)
	if err != nil {
		return nil, err
	}

	ms.EnqueueMsg(ctx, queuedMsg)

	ctx.GasMeter().ConsumeGas(ms.GetParams(ctx).EnqueueGasFees.Undelegate, "epoching undelegate enqueue fee")

	err = ctx.EventManager().EmitTypedEvents(
		&types.EventWrappedUndelegate{
			DelegatorAddress: msg.Msg.DelegatorAddress,
			ValidatorAddress: msg.Msg.ValidatorAddress,
			Amount:           msg.Msg.Amount.Amount.Uint64(),
			Denom:            msg.Msg.Amount.GetDenom(),
			EpochBoundary:    ms.GetEpoch(ctx).GetLastBlockHeight(),
		},
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgWrappedUndelegateResponse{}, nil
}

// WrappedBeginRedelegate handles the MsgWrappedBeginRedelegate request
func (ms msgServer) WrappedBeginRedelegate(goCtx context.Context, msg *types.MsgWrappedBeginRedelegate) (*types.MsgWrappedBeginRedelegateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Msg == nil {
		return nil, types.ErrNoWrappedMsg
	}

	// verification rules ported from staking module
	valSrcAddr, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorSrcAddress)
	if err != nil {
		return nil, err
	}
	delegatorAddress, err := sdk.AccAddressFromBech32(msg.Msg.DelegatorAddress)
	if err != nil {
		return nil, err
	}
	if _, err := ms.stk.ValidateUnbondAmount(ctx, delegatorAddress, valSrcAddr, msg.Msg.Amount.Amount); err != nil {
		return nil, err
	}
	bondDenom, err := ms.stk.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	if msg.Msg.Amount.Denom != bondDenom {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "invalid coin denomination: got %s, expected %s", msg.Msg.Amount.Denom, bondDenom,
		)
	}

	params := ms.GetParams(ctx)
	if msg.Msg.Amount.Amount.LT(math.NewIntFromUint64(params.MinAmount)) {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"redelegation amount %s is below minimum required amount %d",
			msg.Msg.Amount.Amount.String(),
			params.MinAmount,
		)
	}

	if _, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorDstAddress); err != nil {
		return nil, err
	}

	blockHeight := uint64(ctx.HeaderInfo().Height)
	if blockHeight == 0 {
		return nil, types.ErrZeroEpochMsg
	}
	blockTime := ctx.HeaderInfo().Time

	txid := tmhash.Sum(ctx.TxBytes())
	queuedMsg, err := types.NewQueuedMessage(blockHeight, blockTime, txid, msg)
	if err != nil {
		return nil, err
	}

	ms.EnqueueMsg(ctx, queuedMsg)

	ctx.GasMeter().ConsumeGas(ms.GetParams(ctx).EnqueueGasFees.BeginRedelegate, "epoching Redelegate enqueue fee")

	err = ctx.EventManager().EmitTypedEvents(
		&types.EventWrappedBeginRedelegate{
			DelegatorAddress:            msg.Msg.DelegatorAddress,
			SourceValidatorAddress:      msg.Msg.ValidatorSrcAddress,
			DestinationValidatorAddress: msg.Msg.ValidatorDstAddress,
			Amount:                      msg.Msg.Amount.Amount.Uint64(),
			Denom:                       msg.Msg.Amount.GetDenom(),
			EpochBoundary:               ms.GetEpoch(ctx).GetLastBlockHeight(),
		},
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgWrappedBeginRedelegateResponse{}, nil
}

// WrappedCancelUnbondingDelegation handles the MsgWrappedCancelUnbondingDelegation request
func (ms msgServer) WrappedCancelUnbondingDelegation(goCtx context.Context, msg *types.MsgWrappedCancelUnbondingDelegation) (*types.MsgWrappedCancelUnbondingDelegationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Msg == nil {
		return nil, types.ErrNoWrappedMsg
	}

	// verification rules ported from staking module
	if _, err := sdk.AccAddressFromBech32(msg.Msg.DelegatorAddress); err != nil {
		return nil, err
	}

	if _, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorAddress); err != nil {
		return nil, err
	}

	if !msg.Msg.Amount.IsValid() || !msg.Msg.Amount.Amount.IsPositive() {
		return nil, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"invalid amount",
		)
	}

	params := ms.GetParams(ctx)
	if msg.Msg.Amount.Amount.LT(math.NewIntFromUint64(params.MinAmount)) {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"cancel unbonding delegaion amount %s is below minimum required amount %d",
			msg.Msg.Amount.Amount.String(),
			params.MinAmount,
		)
	}

	if msg.Msg.CreationHeight <= 0 {
		return nil, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"invalid height",
		)
	}

	bondDenom, err := ms.stk.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	if msg.Msg.Amount.Denom != bondDenom {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "invalid coin denomination: got %s, expected %s", msg.Msg.Amount.Denom, bondDenom,
		)
	}

	blockHeight := uint64(ctx.HeaderInfo().Height)
	if blockHeight == 0 {
		return nil, types.ErrZeroEpochMsg
	}
	blockTime := ctx.BlockTime()
	txid := tmhash.Sum(ctx.TxBytes())
	queuedMsg, err := types.NewQueuedMessage(blockHeight, blockTime, txid, msg)
	if err != nil {
		return nil, err
	}

	ms.EnqueueMsg(ctx, queuedMsg)

	ctx.GasMeter().ConsumeGas(ms.GetParams(ctx).EnqueueGasFees.CancelUnbondingDelegation, "epoching cancel unbonding delegation enqueue fee")

	err = ctx.EventManager().EmitTypedEvents(
		&types.EventWrappedCancelUnbondingDelegation{
			DelegatorAddress: msg.Msg.DelegatorAddress,
			ValidatorAddress: msg.Msg.ValidatorAddress,
			Amount:           msg.Msg.Amount.Amount.Uint64(),
			CreationHeight:   msg.Msg.CreationHeight,
			EpochBoundary:    ms.GetEpoch(ctx).GetLastBlockHeight(),
		},
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgWrappedCancelUnbondingDelegationResponse{}, nil
}

// UpdateParams updates the params.
// TODO investigate when it is the best time to update the params. We can update them
// when the epoch changes, but we can also update them during the epoch and extend
// the epoch duration.
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
