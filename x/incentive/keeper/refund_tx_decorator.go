package keeper

import (
	"bytes"
	"errors"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

var _ sdk.PostDecorator = &RefundTxDecorator{}
var _ sdk.AnteDecorator = &RefundTxDecorator{}

type RefundTxDecorator struct {
	k                   *Keeper
	ak                  anteinterfaces.AccountKeeper
	fmk                 *feemarketkeeper.Keeper
	currentTxInitialGas uint64
}

// NewRefundTxDecorator creates a new RefundTxDecorator
func NewRefundTxDecorator(k *Keeper, ak anteinterfaces.AccountKeeper, fmk *feemarketkeeper.Keeper) *RefundTxDecorator {
	return &RefundTxDecorator{
		k:   k,
		ak:  ak,
		fmk: fmk,
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

	// calculate gas consumed by tx size at cosmos ante handler
	params := d.ak.GetParams(ctx)
	gasConsumedByTxSize := params.TxSizeCostPerByte * storetypes.Gas(len(ctx.TxBytes()))

	// calculate gas consumed by signature verification at cosmos ante handler
	var gasConsumedBySigVerification uint64
	gasConsumedBySigVerification += params.SigVerifyCostSecp256k1

	d.currentTxInitialGas = ctx.GasMeter().GasConsumed() - gasConsumedByTxSize - gasConsumedBySigVerification

	return next(ctx, tx, simulate)
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

	refundable := d.CheckTxAndClearIndex(ctx, tx)

	// if the tx is refundable, refund it
	if refundable {
		err := d.k.RefundTx(ctx, feeTx)
		if err != nil {
			d.k.Logger(ctx).Error("failed to refund tx", "error", err)
			return next(ctx, tx, simulate, success)
		}

		totalGasConsumed := ctx.GasMeter().GasConsumed() - d.currentTxInitialGas

		// refund gas consumed from tx gas meter
		ctx.GasMeter().RefundGas(totalGasConsumed, "refundable babylon tx gas consumption refund")

		// refund gas wanted from transient gas wanted
		if d.fmk != nil {
			feeTx, ok := tx.(sdk.FeeTx)
			if !ok {
				return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
			}
			gasLimit := feeTx.GetGas()
			currentGasWanted := d.fmk.GetTransientGasWanted(ctx)
			if currentGasWanted < gasLimit {
				return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx gas limit is greater than the current gas wanted")
			}
			adjustedGasWanted := currentGasWanted - gasLimit
			d.fmk.SetTransientBlockGasWanted(ctx, adjustedGasWanted)
		}

		// reset tracking
		d.currentTxInitialGas = 0
	}

	// move to the next PostHandler
	return next(ctx, tx, simulate, success)
}

// CheckTxAndClearIndex parses the given tx and returns a boolean indicating
// whether the tx is refundable, and clears the refundable messages from the index
// NOTE: a tx is refundable if all of its messages are unique and refundable
func (d *RefundTxDecorator) CheckTxAndClearIndex(ctx sdk.Context, tx sdk.Tx) bool {
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

	// if all messages in the tx are unique and refundable, then this tx is refundable
	refundable := len(refundableMsgHashSet) == len(tx.GetMsgs())

	// remove the refundable messages from the index, regardless whether the tx is refunded or not
	// NOTE: If the message with same hash shows up in a later tx, the refunding rule will
	// consider it duplicated and the tx will not be refunded
	for msgHash := range refundableMsgHashSet {
		d.k.RemoveRefundableMsg(ctx, []byte(msgHash))
	}

	return refundable
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
