package ante

import (
	"context"

	"cosmossdk.io/core/address"
	"cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	txsigning "cosmossdk.io/x/tx/signing"
	wasmapp "github.com/CosmWasm/wasmd/app"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcckeeper "github.com/babylonlabs-io/babylon/x/btccheckpoint/keeper"
	epochingkeeper "github.com/babylonlabs-io/babylon/x/epoching/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
	evmkeeper "github.com/evmos/ethermint/x/evm/keeper"
	feemarketkeeper "github.com/evmos/ethermint/x/feemarket/keeper"
)

type AccountKeeper interface {
	GetParams(ctx context.Context) (params types.Params)
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx context.Context, acc sdk.AccountI)
	GetModuleAddress(moduleName string) sdk.AccAddress
	AddressCodec() address.Codec
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetAllAccounts(ctx context.Context) (accounts []sdk.AccountI)
	IterateAccounts(ctx context.Context, cb func(account sdk.AccountI) bool)
	GetSequence(context.Context, sdk.AccAddress) (uint64, error)
	RemoveAccount(ctx context.Context, account sdk.AccountI)
}

type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	IsSendEnabledCoins(ctx context.Context, coins ...sdk.Coin) error
	SendCoins(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BlockedAddr(addr sdk.AccAddress) bool
}

// NewAnteHandler creates a new AnteHandler for the Babylon chain.
func NewAnteHandler(
	accountKeeper AccountKeeper,
	bankKeeper BankKeeper,
	feegrantKeeper authante.FeegrantKeeper,
	signModeHandler *txsigning.HandlerMap,
	ibcKeeper *ibckeeper.Keeper,
	wasmConfig *wasmtypes.WasmConfig,
	wasmKeeper *wasmkeeper.Keeper,
	circuitKeeper *circuitkeeper.Keeper,
	epochingKeeper *epochingkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
	feemarketKeeper feemarketkeeper.Keeper,
	btcConfig *bbn.BtcConfig,
	btccKeeper *btcckeeper.Keeper,
	txCounterStoreService store.KVStoreService,
) sdk.AnteHandler {
	// initialize Babylon auth ante handler
	authAnteHandler, err := wasmapp.NewAnteHandler(
		wasmapp.HandlerOptions{
			HandlerOptions: authante.HandlerOptions{
				AccountKeeper:   accountKeeper,
				BankKeeper:      bankKeeper,
				SignModeHandler: signModeHandler,
				FeegrantKeeper:  feegrantKeeper,
				SigGasConsumer:  authante.DefaultSigVerificationGasConsumer,
				TxFeeChecker:    CheckTxFeeWithGlobalMinGasPrices,
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

	// Initialize EVM ante handler
	ethAnteHandler := NewEVMAnteHandler(
		accountKeeper,
		bankKeeper,
		feegrantKeeper,
		signModeHandler,
		ibcKeeper,
		evmKeeper,
		feemarketKeeper,
	)

	// Create a chained AnteHandler for Babylon Cosmos transactions
	babylonCosmosAnteHandler := sdk.ChainAnteDecorators(
		NewWrappedAnteHandler(authAnteHandler),
		epochingkeeper.NewDropValidatorMsgDecorator(epochingKeeper),
		NewBtcValidationDecorator(btcConfig, btccKeeper),
	)

	return func(
		ctx sdk.Context, tx sdk.Tx, sim bool,
	) (newCtx sdk.Context, err error) {
		var anteHandler sdk.AnteHandler

		// disable vesting message types
		for _, msg := range tx.GetMsgs() {
			switch msg.(type) {
			case *vestingtypes.MsgCreateVestingAccount,
				*vestingtypes.MsgCreatePeriodicVestingAccount,
				*vestingtypes.MsgCreatePermanentLockedAccount:
				return ctx, errorsmod.Wrapf(
					errortypes.ErrInvalidRequest,
					"vesting messages are not supported",
				)
			}
		}

		// Check for EVM extension option
		if evmAnteHandler, err := CheckEVMExtensionOption(tx, ethAnteHandler); err != nil {
			return ctx, err
		} else if evmAnteHandler != nil {
			return evmAnteHandler(ctx, tx, sim)
		}

		// handle as totally normal Cosmos SDK tx
		switch tx.(type) {
		case sdk.Tx:
			anteHandler = babylonCosmosAnteHandler
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
