package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
)

type AppAnteHandler struct {
	bypassGasMeter          BypassGasDecorator
	validateInternalMsg     ValidateInternalMsgDecorator
	extensionOptionsChecker sdk.AnteDecorator
	txTimeoutHeight         sdk.AnteDecorator
	validateMemo            sdk.AnteDecorator
}

func NewAppAnteHandler(accountKeeper authante.AccountKeeper) AppAnteHandler {
	return AppAnteHandler{
		bypassGasMeter:          NewBypassGasDecorator(),
		validateInternalMsg:     NewValidateInternalMsgDecorator(),
		extensionOptionsChecker: authante.NewExtensionOptionsDecorator(nil),
		txTimeoutHeight:         authante.NewTxTimeoutHeightDecorator(),
		validateMemo:            authante.NewValidateMemoDecorator(accountKeeper),
	}
}

func emptyAnteHandler(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
	return ctx, nil
}

func (ah AppAnteHandler) appInjectedMsgAnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool) (
	newCtx sdk.Context,
	err error,
) {
	ctx, err = ah.bypassGasMeter.AnteHandle(ctx, tx, simulate, emptyAnteHandler)
	if err != nil {
		return ctx, err
	}

	ctx, err = ah.extensionOptionsChecker.AnteHandle(ctx, tx, simulate, emptyAnteHandler)
	if err != nil {
		return ctx, err
	}

	ctx, err = ah.txTimeoutHeight.AnteHandle(ctx, tx, simulate, emptyAnteHandler)
	if err != nil {
		return ctx, err
	}

	ctx, err = ah.validateInternalMsg.AnteHandle(ctx, tx, simulate, emptyAnteHandler)
	if err != nil {
		return ctx, err
	}

	return ctx, err
}
