package keeper

import (
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RefundTx refunds the given tx by sending the fee back to the fee payer.
func (k Keeper) RefundTx(ctx sdk.Context, tx sdk.FeeTx) error {
	txFee := tx.GetFee()
	txFeePayer := tx.FeePayer()

	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, k.feeCollectorName, txFeePayer, txFee)
}

// IndexRefundableMsg indexes the given refundable message by its hash.
func (k Keeper) IndexRefundableMsg(ctx sdk.Context, msg sdk.Msg) {
	msgHash := types.HashMsg(msg)
	err := k.RefundableMsgKeySet.Set(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}

// HasRefundableMsg checks if the message with a given hash is refundable.
func (k Keeper) HasRefundableMsg(ctx sdk.Context, msgHash []byte) bool {
	has, err := k.RefundableMsgKeySet.Has(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
	return has
}

// RemoveRefundableMsg removes the given refundable message from the index.
func (k Keeper) RemoveRefundableMsg(ctx sdk.Context, msgHash []byte) {
	err := k.RefundableMsgKeySet.Remove(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}
