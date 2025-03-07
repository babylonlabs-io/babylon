package ante

import (
	"github.com/babylonlabs-io/babylon/app/ante/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BypassGasDecorator struct {
}

func NewBypassGasDecorator() BypassGasDecorator {
	return BypassGasDecorator{}
}

func (bd BypassGasDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	if SingleInjectedMsg(tx.GetMsgs()) {
		newCtx = ctx.WithBlockGasMeter(types.NewBypassGasMeter())
		return next(newCtx, tx, simulate)
	}
	return next(ctx, tx, simulate)
}
