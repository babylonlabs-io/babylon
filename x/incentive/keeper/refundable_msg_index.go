package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// getRecipient returns the address that should receive the refund.
func getRecipient(feeTx sdk.FeeTx) sdk.AccAddress {
	if feeGranter := feeTx.FeeGranter(); feeGranter != nil {
		return feeGranter
	}
	return feeTx.FeePayer()
}

// RefundTx refunds the given tx by sending the fee back to the fee payer.
func (k Keeper) RefundTx(ctx context.Context, tx sdk.FeeTx) error {
	txFee := tx.GetFee()
	if txFee.IsZero() {
		// not possible with the global min gas price mechanism
		// but having this check for compatibility in the future
		return nil
	}
	txFeePayer := getRecipient(tx)

	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, k.feeCollectorName, txFeePayer, txFee)
}

// IndexRefundableMsg indexes the given refundable message by its hash.
func (k Keeper) IndexRefundableMsg(ctx context.Context, msg sdk.Msg) {
	msgHash := types.HashMsg(msg)
	err := k.RefundableMsgKeySet.Set(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}

// HasRefundableMsg checks if the message with a given hash is refundable.
func (k Keeper) HasRefundableMsg(ctx context.Context, msgHash []byte) bool {
	has, err := k.RefundableMsgKeySet.Has(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
	return has
}

// RemoveRefundableMsg removes the given refundable message from the index.
func (k Keeper) RemoveRefundableMsg(ctx context.Context, msgHash []byte) {
	err := k.RefundableMsgKeySet.Remove(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}

// IncRefundableMsgCount increment RefundableMsgCount by one
func (k *Keeper) IncRefundableMsgCount() {
	k.RefundableMsgCount++
}

// ResetRefundableMsgCount reset RefundableMsgCount to zero
func (k *Keeper) ResetRefundableMsgCount() {
	k.RefundableMsgCount = 0
}
