package ante

import (
	errorsmod "cosmossdk.io/errors"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/spf13/cast"
)

// DefaultMaxGasWantedPerTx is the default value for the maximum gas allowed for a single transaction
const DefaultMaxGasWantedPerTx = 10_000_000

type MempoolOptions struct {
	MaxGasWantedPerTx uint64
}

func NewDefaultMempoolOptions() MempoolOptions {
	return MempoolOptions{
		MaxGasWantedPerTx: DefaultMaxGasWantedPerTx,
	}
}

func NewMempoolOptions(opts servertypes.AppOptions) MempoolOptions {
	return MempoolOptions{
		MaxGasWantedPerTx: parseMaxGasWantedPerTx(opts),
	}
}

// GasLimitDecorator will check if the gas required by a transaction
// is less than the defined MaxGasWantedPerTx
type GasLimitDecorator struct {
	maxGasWantedPerTx uint64
}

func NewGasLimitDecorator(opts MempoolOptions) GasLimitDecorator {
	return GasLimitDecorator{
		maxGasWantedPerTx: opts.MaxGasWantedPerTx,
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

func parseMaxGasWantedPerTx(opts servertypes.AppOptions) uint64 {
	valueInterface := opts.Get("babylon-mempool.max-gas-wanted-per-tx")
	if valueInterface == nil {
		return DefaultMaxGasWantedPerTx
	}
	value, err := cast.ToUint64E(valueInterface)
	if err != nil {
		panic("invalidly configured babylon-mempool.max-gas-wanted-per-tx")
	}
	return value
}
