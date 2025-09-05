package keeper

import (
	"encoding/hex"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// lockFunds transfers funds from an account to the delegation pool module account
func (k Keeper) lockFunds(ctx sdk.Context, msgId string, fromAddr sdk.AccAddress, coins sdk.Coins) error {
	if err := k.bk.SendCoinsFromAccountToModule(ctx, fromAddr, types.DelegatePoolModuleName, coins); err != nil {
		return errorsmod.Wrapf(err, "failed to lock funds for msg %s", msgId)
	}
	return nil
}

// unlockFunds transfers funds from the delegation pool module account to an account
func (k Keeper) unlockFunds(ctx sdk.Context, msgId string, toAddr sdk.AccAddress, coins sdk.Coins) error {
	if err := k.bk.SendCoinsFromModuleToAccount(ctx, types.DelegatePoolModuleName, toAddr, coins); err != nil {
		return errorsmod.Wrapf(err, "failed to unlock funds for msg %s", msgId)
	}
	return nil
}

// LockFunds locks user funds in delegate pool until the message is executed
func (k Keeper) LockFundsForDelegateMsgs(ctx sdk.Context, msg *types.QueuedMessage) error {
	msgId := hex.EncodeToString(msg.MsgId)
	switch wrappedMsg := msg.Msg.(type) {
	case *types.QueuedMessage_MsgDelegate:
		delegatorAddr, err := sdk.AccAddressFromBech32(wrappedMsg.MsgDelegate.DelegatorAddress)
		if err != nil {
			return err
		}
		coins := sdk.NewCoins(wrappedMsg.MsgDelegate.Amount)
		return k.lockFunds(ctx, msgId, delegatorAddr, coins)
	case *types.QueuedMessage_MsgCreateValidator:
		validatorAddr, err := sdk.ValAddressFromBech32(wrappedMsg.MsgCreateValidator.ValidatorAddress)
		if err != nil {
			return err
		}
		valAddr := sdk.AccAddress(validatorAddr)
		coins := sdk.NewCoins(wrappedMsg.MsgCreateValidator.Value)
		return k.lockFunds(ctx, msgId, valAddr, coins)
	default:
		// No fund locking needed for other message types
		return nil
	}
}

// UnLockFunds unlocks user funds from delegate pool when the message is executed
func (k Keeper) UnlockFundsForDelegateMsgs(ctx sdk.Context, msg *types.QueuedMessage) error {
	msgId := hex.EncodeToString(msg.MsgId)
	switch wrappedMsg := msg.Msg.(type) {
	case *types.QueuedMessage_MsgDelegate:
		delegatorAddr, err := sdk.AccAddressFromBech32(wrappedMsg.MsgDelegate.DelegatorAddress)
		if err != nil {
			return err
		}
		coins := sdk.NewCoins(wrappedMsg.MsgDelegate.Amount)
		return k.unlockFunds(ctx, msgId, delegatorAddr, coins)
	case *types.QueuedMessage_MsgCreateValidator:
		validatorAddr, err := sdk.ValAddressFromBech32(wrappedMsg.MsgCreateValidator.ValidatorAddress)
		if err != nil {
			return err
		}
		valAddr := sdk.AccAddress(validatorAddr)
		coins := sdk.NewCoins(wrappedMsg.MsgCreateValidator.Value)
		return k.unlockFunds(ctx, msgId, valAddr, coins)
	default:
		// No fund locking needed for other message types
		return nil
	}
}
