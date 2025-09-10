package keeper

import (
	"encoding/hex"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// extractAddrAndCoinsFromMsg extracts address and coins from QueuedMessage
func (k Keeper) extractAddrAndCoinsFromMsg(msg *types.QueuedMessage) (sdk.AccAddress, sdk.Coins, string, error) {
	msgId := hex.EncodeToString(msg.MsgId)
	switch wrappedMsg := msg.Msg.(type) {
	case *types.QueuedMessage_MsgDelegate:
		delegatorAddr, err := sdk.AccAddressFromBech32(wrappedMsg.MsgDelegate.DelegatorAddress)
		if err != nil {
			return nil, nil, msgId, err
		}
		coins := sdk.NewCoins(wrappedMsg.MsgDelegate.Amount)
		return delegatorAddr, coins, msgId, nil
	case *types.QueuedMessage_MsgCreateValidator:
		validatorAddr, err := sdk.ValAddressFromBech32(wrappedMsg.MsgCreateValidator.ValidatorAddress)
		if err != nil {
			return nil, nil, msgId, err
		}
		valAddr := sdk.AccAddress(validatorAddr)
		coins := sdk.NewCoins(wrappedMsg.MsgCreateValidator.Value)
		return valAddr, coins, msgId, nil
	default:
		// No fund locking needed for other message types
		return nil, nil, msgId, nil
	}
}

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
	addr, coins, msgId, err := k.extractAddrAndCoinsFromMsg(msg)
	if err != nil {
		return err
	}
	if addr == nil && coins == nil {
		// No fund locking needed for this message type
		return nil
	}
	return k.lockFunds(ctx, msgId, addr, coins)
}

// UnLockFunds unlocks user funds from delegate pool when the message is executed
func (k Keeper) UnlockFundsForDelegateMsgs(ctx sdk.Context, msg *types.QueuedMessage) error {
	addr, coins, msgId, err := k.extractAddrAndCoinsFromMsg(msg)
	if err != nil {
		return err
	}
	if addr == nil && coins == nil {
		// No fund unlocking needed for this message type
		return nil
	}
	return k.unlockFunds(ctx, msgId, addr, coins)
}
