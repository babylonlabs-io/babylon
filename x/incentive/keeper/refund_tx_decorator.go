package keeper

import (
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.PostDecorator = &RefundTxDecorator{}

type RefundTxDecorator struct {
	k *Keeper
}

// NewRefundTxDecorator creates a new RefundTxDecorator
func NewRefundTxDecorator(k *Keeper) *RefundTxDecorator {
	return &RefundTxDecorator{
		k: k,
	}
}

func (d *RefundTxDecorator) PostHandle(ctx sdk.Context, tx sdk.Tx, simulate, success bool, next sdk.PostHandler) (sdk.Context, error) {
	// refund only when finalizing a block or simulating the current tx
	if ctx.ExecMode() != sdk.ExecModeFinalize && !simulate {
		return next(ctx, tx, simulate, success)
	}
	// ignore unsuccessful tx
	// NOTE: tx with a misbehaving header will still succeed, but will make the client to be frozen
	if !success {
		return next(ctx, tx, simulate, success)
	}
	// ignore tx that does not need to pay fee
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return next(ctx, tx, simulate, success)
	}

	refundableMsgHashList := make([][]byte, 0)

	// iterate over all messages in the tx, and record whether they are refundable
	for _, msg := range tx.GetMsgs() {
		msgHash := types.HashMsg(msg)
		if d.k.HasRefundableMsg(ctx, msgHash) {
			refundableMsgHashList = append(refundableMsgHashList, msgHash)
		}
	}

	// if all messages in the tx are refundable, refund the tx
	if len(refundableMsgHashList) == len(tx.GetMsgs()) {
		err := d.k.RefundTx(ctx, feeTx)
		if err != nil {
			d.k.Logger(ctx).Error("failed to refund tx", "error", err)
			return next(ctx, tx, simulate, success)
		}
	}

	// remove the refundable messages from the index
	for _, msgHash := range refundableMsgHashList {
		d.k.RemoveRefundableMsg(ctx, msgHash)
	}

	// move to the next PostHandler
	return next(ctx, tx, simulate, success)
}
