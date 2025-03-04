package ante

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// defaultMaxGasWantedPerTx is the maximum gas allowed for a single transaction
const defaultMaxGasWantedPerTx = 50_000_000

// GasLimitDecorator will check if the gas required by a transaction
// is less than the defined MaxGasWantedPerTx
type GasLimitDecorator struct {
	maxGasWantedPerTx uint64
}

func NewGasLimitDecorator() GasLimitDecorator {
	return GasLimitDecorator{
		maxGasWantedPerTx: defaultMaxGasWantedPerTx,
	}
}

func (gld GasLimitDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	// Ensure that the provided gas is less than the maximum gas per tx,
	// if this is a CheckTx. This is only for local mempool purposes, and thus
	// is only ran on check tx.
	if ctx.IsCheckTx() && !simulate {
		if feeTx.GetGas() > gld.maxGasWantedPerTx {
			msg := "Too much gas wanted: %d, maximum is %d"
			return ctx, errorsmod.Wrapf(sdkerrors.ErrOutOfGas, msg, feeTx.GetGas(), gld.maxGasWantedPerTx)
		}
	}

	return next(ctx, tx, simulate)
}
