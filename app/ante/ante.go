package ante

import (
	"cosmossdk.io/core/store"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	txsigning "cosmossdk.io/x/tx/signing"
	wasmapp "github.com/CosmWasm/wasmd/app"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcckeeper "github.com/babylonlabs-io/babylon/x/btccheckpoint/keeper"
	epochingkeeper "github.com/babylonlabs-io/babylon/x/epoching/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
)

// NewAnteHandler creates a new AnteHandler for the Babylon chain.
func NewAnteHandler(
	accountKeeper authante.AccountKeeper,
	bankKeeper authtypes.BankKeeper,
	feegrantKeeper authante.FeegrantKeeper,
	signModeHandler *txsigning.HandlerMap,
	ibcKeeper *ibckeeper.Keeper,
	wasmConfig *wasmtypes.WasmConfig,
	wasmKeeper *wasmkeeper.Keeper,
	circuitKeeper *circuitkeeper.Keeper,
	epochingKeeper *epochingkeeper.Keeper,
	btcConfig *bbn.BtcConfig,
	btccKeeper *btcckeeper.Keeper,
	txCounterStoreService store.KVStoreService,
) sdk.AnteHandler {
	// initialize AnteHandler, which includes
	// - authAnteHandler
	// - custom wasm ante handler NewLimitSimulationGasDecorator and NewCountTXDecorator
	// - Extra decorators introduced in Babylon, such as DropValidatorMsgDecorator that delays validator-related messages
	//
	// We are using constructor from wasmapp as it introduces custom wasm ante handle decorators
	// early in chain of ante handlers.
	authAnteHandler, err := wasmapp.NewAnteHandler(
		wasmapp.HandlerOptions{
			HandlerOptions: authante.HandlerOptions{
				AccountKeeper:   accountKeeper,
				BankKeeper:      bankKeeper,
				SignModeHandler: signModeHandler,
				FeegrantKeeper:  feegrantKeeper,
				SigGasConsumer:  authante.DefaultSigVerificationGasConsumer,
				// CheckTxFeeWithGlobalMinGasPrices will enforce the global minimum
				// gas price for all transactions.
				TxFeeChecker: CheckTxFeeWithGlobalMinGasPrices,
			},
			IBCKeeper:             ibcKeeper,
			WasmConfig:            wasmConfig,
			TXCounterStoreService: txCounterStoreService,
			WasmKeeper:            wasmKeeper,
			CircuitKeeper:         circuitKeeper,
		},
	)

	if err != nil {
		panic(err)
	}

	anteHandler := sdk.ChainAnteDecorators(
		NewWrappedAnteHandler(authAnteHandler),
		NewBtcValidationDecorator(btcConfig, btccKeeper),
	)

	return anteHandler
}

// WrappedAnteHandler is the wrapped AnteHandler that implements the `AnteDecorator` interface, which has a single function `AnteHandle`.
// It allows us to chain an existing AnteHandler with other decorators by using `sdk.ChainAnteDecorators`.
type WrappedAnteHandler struct {
	ah sdk.AnteHandler
}

// NewWrappedAnteHandler creates a new WrappedAnteHandler for a given AnteHandler.
func NewWrappedAnteHandler(ah sdk.AnteHandler) WrappedAnteHandler {
	return WrappedAnteHandler{ah}
}

func (wah WrappedAnteHandler) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	newCtx, err = wah.ah(ctx, tx, simulate)
	if err != nil {
		return newCtx, err
	}
	return next(newCtx, tx, simulate)
}
