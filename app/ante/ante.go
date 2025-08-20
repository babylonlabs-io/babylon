package ante

import (
	"cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	txsigning "cosmossdk.io/x/tx/signing"
	wasmapp "github.com/CosmWasm/wasmd/app"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcckeeper "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/keeper"
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	incentivekeeper "github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	evmante "github.com/cosmos/evm/ante/evm"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
)

// NewAnteHandler creates a new AnteHandler for the Babylon chain.
func NewAnteHandler(
	appOpts servertypes.AppOptions,
	evmHandlerOptions EVMHandlerOptions,
	accountKeeper anteinterfaces.AccountKeeper,
	bankKeeper authtypes.BankKeeper,
	feegrantKeeper authante.FeegrantKeeper,
	signModeHandler *txsigning.HandlerMap,
	ibcKeeper *ibckeeper.Keeper,
	wasmConfig *wasmtypes.NodeConfig,
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
			NodeConfig:            wasmConfig,
			TXCounterStoreService: txCounterStoreService,
			WasmKeeper:            wasmKeeper,
			CircuitKeeper:         circuitKeeper,
		},
	)

	if err != nil {
		panic(err)
	}

	mempoolOpts := NewMempoolOptions(appOpts)
	cosmosAnteHandler := sdk.ChainAnteDecorators(
		NewGasLimitDecorator(mempoolOpts),
		NewIBCMsgSizeDecorator(),
		NewWrappedAnteHandler(authAnteHandler),
		evmante.NewGasWantedDecorator(evmHandlerOptions.EvmKeeper, evmHandlerOptions.FeeMarketKeeper),
		NewBtcValidationDecorator(btcConfig, btccKeeper),
		incentivekeeper.NewRefundTxDecorator(nil, nil),
		NewPriorityDecorator(),
	)

	return routeAnteHandler(evmHandlerOptions, cosmosAnteHandler)
}

// routeAnteHandler routes transactions to the appropriate ante handler based on extension options
func routeAnteHandler(
	evmHandlerOptions EVMHandlerOptions,
	cosmosAnteHandler sdk.AnteHandler,
) sdk.AnteHandler {
	return func(
		ctx sdk.Context, tx sdk.Tx, sim bool,
	) (newCtx sdk.Context, err error) {
		var anteHandler sdk.AnteHandler

		txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
		if ok {
			opts := txWithExtensions.GetExtensionOptions()
			if len(opts) > 0 {
				switch typeURL := opts[0].GetTypeUrl(); typeURL {
				case "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx":
					// handle as *evmtypes.MsgEthereumTx
					anteHandler = newMonoEVMAnteHandler(evmHandlerOptions)
				case "/cosmos.evm.types.v1.ExtensionOptionDynamicFeeTx":
					// cosmos-sdk tx with dynamic fee extension
					anteHandler = cosmosAnteHandler
				default:
					return ctx, errorsmod.Wrapf(
						errortypes.ErrUnknownExtensionOptions,
						"rejecting tx with unsupported extension option: %s", typeURL,
					)
				}

				return anteHandler(ctx, tx, sim)
			}
		}

		// normal Cosmos SDK tx routes back to cosmosAnteHandler
		switch tx.(type) {
		case sdk.Tx:
			anteHandler = cosmosAnteHandler
		default:
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid transaction type: %T", tx)
		}

		return anteHandler(ctx, tx, sim)
	}
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
