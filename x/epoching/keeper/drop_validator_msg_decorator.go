package keeper

import (
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// DropValidatorMsgDecorator defines an AnteHandler decorator that rejects all messages that might change the validator set.
type DropValidatorMsgDecorator struct {
	ek *Keeper
}

// NewDropValidatorMsgDecorator creates a new DropValidatorMsgDecorator
func NewDropValidatorMsgDecorator(ek *Keeper) *DropValidatorMsgDecorator {
	return &DropValidatorMsgDecorator{
		ek: ek,
	}
}

// AnteHandle performs an AnteHandler check that rejects all non-wrapped validator-related messages.
// It will reject the following types of messages:
// - MsgCreateValidator
// - MsgDelegate
// - MsgUndelegate
// - MsgBeginRedelegate
// - MsgCancelUnbondingDelegation
func (qmd DropValidatorMsgDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// skip if at genesis block, as genesis state contains txs that bootstrap the initial validator set
	if ctx.BlockHeight() == 0 {
		return next(ctx, tx, simulate)
	}
	// after genesis, if validator-related message, reject msg
	for _, msg := range tx.GetMsgs() {
		if err := qmd.ValidateMsg(msg); err != nil {
			return sdk.Context{}, err
		}
	}

	return next(ctx, tx, simulate)
}

// ValidateMsg checks if the given message is of non-wrapped type, which should be rejected
// It returns true if the message is a validator-related message, and false otherwise.
// It returns an error if it is a MsgExec message which contains invalid messages.
func (qmd DropValidatorMsgDecorator) ValidateMsg(msg sdk.Msg) error {
	switch msg := msg.(type) {
	case *stakingtypes.MsgCreateValidator, *stakingtypes.MsgDelegate, *stakingtypes.MsgUndelegate, *stakingtypes.MsgBeginRedelegate, *stakingtypes.MsgCancelUnbondingDelegation:
		// validator-related message
		return epochingtypes.ErrUnwrappedMsgType
	case *authz.MsgExec:
		// MsgExec might contain a validator-related message and those should
		// not bypass the ante handler
		// https://jumpcrypto.com/writing/bypassing-ethermint-ante-handlers/
		// unpack the exec message
		internalMsgs, err := msg.GetMessages()
		if err != nil {
			// the internal message is not valid
			return err
		}
		// check if any of the internal messages is a validator-related message
		for _, internalMsg := range internalMsgs {
			// recursively validate the internal message
			if err := qmd.ValidateMsg(internalMsg); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}
