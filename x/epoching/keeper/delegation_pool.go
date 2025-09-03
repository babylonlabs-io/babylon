package keeper

import (
	"encoding/hex"

	errorsmod "cosmossdk.io/errors"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LockUserFunds locks user funds in delegate pool during message enqueue

func (k Keeper) LockFunds(ctx sdk.Context, msg *types.QueuedMessage) error {
	msgId := hex.EncodeToString(msg.MsgId)
	switch wrappedMsg := msg.Msg.(type) {
	case *types.QueuedMessage_MsgDelegate:
		wrappedDelegate := &types.MsgWrappedDelegate{
			Msg: wrappedMsg.MsgDelegate,
		}
		return k.lockDelegateFunds(ctx, msgId, wrappedDelegate)
	case *types.QueuedMessage_MsgCreateValidator:
		wrappedCreateValidator := &checkpointingtypes.MsgWrappedCreateValidator{
			MsgCreateValidator: wrappedMsg.MsgCreateValidator,
		}
		return k.lockValidatorFunds(ctx, msgId, wrappedCreateValidator)
	default:
		// No fund locking needed for other message types
		return nil
	}
}

func (k Keeper) UnLockFunds(ctx sdk.Context, msg *types.QueuedMessage) error {
	msgId := hex.EncodeToString(msg.MsgId)
	switch wrappedMsg := msg.Msg.(type) {
	case *types.QueuedMessage_MsgDelegate:
		wrappedDelegate := &types.MsgWrappedDelegate{
			Msg: wrappedMsg.MsgDelegate,
		}
		return k.unlockDelegateFunds(ctx, msgId, wrappedDelegate)
	case *types.QueuedMessage_MsgCreateValidator:
		wrappedCreateValidator := &checkpointingtypes.MsgWrappedCreateValidator{
			MsgCreateValidator: wrappedMsg.MsgCreateValidator,
		}
		return k.unlockValidatorFunds(ctx, msgId, wrappedCreateValidator)
	default:
		// No fund locking needed for other message types
		return nil
	}
}

func (k Keeper) lockDelegateFunds(ctx sdk.Context, msgId string, msg *types.MsgWrappedDelegate) error {
	delegatorAddr, err := sdk.AccAddressFromBech32(msg.Msg.DelegatorAddress)
	if err != nil {
		return err
	}

	// Transfer funds from user to delegate pool
	coins := sdk.NewCoins(msg.Msg.Amount)
	if err := k.bk.SendCoinsFromAccountToModule(ctx, delegatorAddr, types.DelegatePoolModuleName, coins); err != nil {
		return errorsmod.Wrapf(err, "failed to lock delegate funds for msg %s", msgId)
	}

	return nil
}

func (k Keeper) lockValidatorFunds(ctx sdk.Context, msgId string, msg *checkpointingtypes.MsgWrappedCreateValidator) error {
	validatorAddr, err := sdk.ValAddressFromBech32(msg.MsgCreateValidator.ValidatorAddress)
	if err != nil {
		return err
	}
	valAddr := sdk.AccAddress(validatorAddr)

	// Transfer funds from user to delegate pool
	coins := sdk.NewCoins(msg.MsgCreateValidator.Value)
	if err := k.bk.SendCoinsFromAccountToModule(ctx, valAddr, types.DelegatePoolModuleName, coins); err != nil {
		return errorsmod.Wrapf(err, "failed to lock delegate funds for msg %s", msgId)
	}

	return nil
}

func (k Keeper) unlockDelegateFunds(ctx sdk.Context, msgId string, msg *types.MsgWrappedDelegate) error {
	delegatorAddr, err := sdk.AccAddressFromBech32(msg.Msg.DelegatorAddress)
	if err != nil {
		return err
	}
	// Transfer funds back from delegate pool to user
	coins := sdk.NewCoins(msg.Msg.Amount)
	if err := k.bk.SendCoinsFromModuleToAccount(ctx, types.DelegatePoolModuleName, delegatorAddr, coins); err != nil {
		return errorsmod.Wrapf(err, "failed to unlock delegate funds for msg %s", msgId)
	}

	// Remove locked fund record
	return nil
}

func (k Keeper) unlockValidatorFunds(ctx sdk.Context, msgId string, msg *checkpointingtypes.MsgWrappedCreateValidator) error {
	validatorAddr, err := sdk.ValAddressFromBech32(msg.MsgCreateValidator.ValidatorAddress)
	if err != nil {
		return err
	}
	valAddr := sdk.AccAddress(validatorAddr)
	// Transfer funds back from delegate pool to user
	coins := sdk.NewCoins(msg.MsgCreateValidator.Value)
	if err := k.bk.SendCoinsFromModuleToAccount(ctx, types.DelegatePoolModuleName, valAddr, coins); err != nil {
		return errorsmod.Wrapf(err, "failed to unlock delegate funds for msg %s", msgId)
	}

	// Remove locked fund record
	return nil
}
