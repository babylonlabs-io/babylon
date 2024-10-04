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
	// avoid refunding failed tx
	if !success {
		return next(ctx, tx, simulate, success)
	}
	// ignore tx that does not need to pay fee
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return next(ctx, tx, simulate, success)
	}

	// NOTE: we use a map to avoid duplicated refundable messages, as
	// otherwise one can fill a tx with duplicate messages to bloat the blockchain
	refundableMsgHashSet := make(map[string]struct{})

	// iterate over all messages in the tx, and record whether they are refundable
	for _, msg := range tx.GetMsgs() {
		msgHash := types.HashMsg(msg)
		if d.k.HasRefundableMsg(ctx, msgHash) {
			refundableMsgHashSet[string(msgHash)] = struct{}{}
		}
	}

	// if all messages in the tx are unique and refundable, refund the tx
	if len(refundableMsgHashSet) == len(tx.GetMsgs()) {
		err := d.k.RefundTx(ctx, feeTx)
		if err != nil {
			d.k.Logger(ctx).Error("failed to refund tx", "error", err)
			return next(ctx, tx, simulate, success)
		}
	}

	// remove the refundable messages from the index, regardless whether the tx is refunded or not
	// NOTE: If the message with same hash shows up again, the refunding rule will
	// consider it duplicated and the tx will not be refunded
	for msgHash := range refundableMsgHashSet {
		d.k.RemoveRefundableMsg(ctx, []byte(msgHash))
	}

	// move to the next PostHandler
	return next(ctx, tx, simulate, success)
}
