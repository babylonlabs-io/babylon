package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

// IncRefundableMsgCount increment RefundableMsgCount by one
func (k *Keeper) IncRefundableMsgCount() {
	k.RefundableMsgCount++
}

// ResetRefundableMsgCount reset RefundableMsgCount to zero
func (k *Keeper) ResetRefundableMsgCount() {
	k.RefundableMsgCount = 0
}
