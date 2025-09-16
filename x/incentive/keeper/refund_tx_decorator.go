package keeper

import (
	"bytes"
	"errors"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/feemarketwrapper"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

var _ sdk.PostDecorator = &RefundTxDecorator{}
var _ sdk.AnteDecorator = &RefundTxDecorator{}

type RefundTxDecorator struct {
	k    *Keeper
	tKey *storetypes.TransientStoreKey
}

// NewRefundTxDecorator creates a new RefundTxDecorator
func NewRefundTxDecorator(k *Keeper, tKey *storetypes.TransientStoreKey) *RefundTxDecorator {
	return &RefundTxDecorator{
		k:    k,
		tKey: tKey,
	}
}

// Makes sure there's no fee granter assigned in refundable txs
// so there's no need to restore allowances
func (d *RefundTxDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if !isRefundTx(tx) {
		return next(ctx, tx, simulate)
	}
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	// If there's a fee granter, don't allow to perform this tx
	feePayer := feeTx.FeePayer()
	feeGranter := feeTx.FeeGranter()
	if feeGranter != nil {
		if !bytes.Equal(feeGranter, feePayer) {
			return ctx, errors.New("it is not possible to use a fee grant in a refundable transaction")
		}
	}

	return next(ctx, tx, simulate)
}

func (d *RefundTxDecorator) PostHandle(ctx sdk.Context, tx sdk.Tx, simulate, success bool, next sdk.PostHandler) (sdk.Context, error) {
	// reset RefundableMsgCount to zero since RefundableMsgCount maintained per-tx
	// Note: to make sure refundable msg count is reset after each tx, we place the reset counter logic here
	defer d.k.ResetRefundableMsgCount()

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

	refundable := d.CheckTxAndClearIndex(ctx, tx)

	// if the tx is refundable, refund it
	if refundable {
		err := d.k.RefundTx(ctx, feeTx)
		if err != nil {
			d.k.Logger(ctx).Error("failed to refund tx", "error", err)
			return next(ctx, tx, simulate, success)
		}

		// set refundable block gas wanted and used for base fee calculation
		feeTx, ok := tx.(sdk.FeeTx)
		if !ok {
			return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
		}
		gasWanted := feeTx.GetGas()
		currentRefundableBlockGasWanted := feemarketwrapper.GetTransientRefundableBlockGasWanted(ctx, d.tKey)
		totalRefundableBlockGasWanted := currentRefundableBlockGasWanted + gasWanted
		feemarketwrapper.SetTransientRefundableBlockGasWanted(ctx, totalRefundableBlockGasWanted, d.tKey)

		/*
			TODO: there are slightly more gas consumed after this posthandle ~327,
				find where that gas is consumed and set refundable gas used at that place
		*/
		gasUsed := ctx.GasMeter().GasConsumed()
		currentRefundableBlockGasUsed := feemarketwrapper.GetTransientRefundableBlockGasUsed(ctx, d.tKey)
		totalRefundableBlockGasUsed := currentRefundableBlockGasUsed + gasUsed
		feemarketwrapper.SetTransientRefundableBlockGasUsed(ctx, totalRefundableBlockGasUsed, d.tKey)
	}

	// move to the next PostHandler
	return next(ctx, tx, simulate, success)
}

// CheckTxAndClearIndex parses the given tx and returns a boolean indicating
// whether the tx is refundable, and clears the refundable messages count to zero
// NOTE: a tx is refundable if all of its messages are unique and refundable
func (d *RefundTxDecorator) CheckTxAndClearIndex(_ sdk.Context, tx sdk.Tx) bool {
	// check if RefundableMsgCount counted up during msgServer, and total msgs in the tx is the same
	// NOTE: this is for the case that message execution doesn't return error even tho it fails
	// you can find these cases in `AddFinalitySig`of x/finality/keeper/msg_server.go
	if len(tx.GetMsgs()) != int(d.k.RefundableMsgCount) {
		return false
	}

	// check if all msgs in the tx is refundable
	if !isRefundTx(tx) {
		return false
	}

	// NOTE: we don't need to check for duplicate refundable msgs in the tx,
	// since it is checked individually in each msg_server.go

	return true
}

// isRefundTx returns true if ALL its messages are refundable
func isRefundTx(tx sdk.Tx) bool {
	if len(tx.GetMsgs()) == 0 {
		return false
	}

	for _, msg := range tx.GetMsgs() {
		switch msg.(type) {
		case *btclctypes.MsgInsertHeaders, // BTC light client
			// BTC timestamping
			*btcctypes.MsgInsertBTCSpvProof,
			// BTC staking
			*bstypes.MsgAddCovenantSigs,
			*bstypes.MsgBTCUndelegate,
			*bstypes.MsgSelectiveSlashingEvidence,
			*bstypes.MsgAddBTCDelegationInclusionProof,
			// BTC staking finality
			*ftypes.MsgAddFinalitySig:
			continue
		default:
			return false
		}
	}
	return true
}
