package ante

import (
	errorsmod "cosmossdk.io/errors"
	txsigning "cosmossdk.io/x/tx/signing"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
	evmante "github.com/evmos/ethermint/app/ante"
	ethermint "github.com/evmos/ethermint/types"
	evmkeeper "github.com/evmos/ethermint/x/evm/keeper"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	feemarketkeeper "github.com/evmos/ethermint/x/feemarket/keeper"
)

func NewEVMAnteHandler(
	accountKeeper AccountKeeper,
	bankKeeper BankKeeper,
	feegrantKeeper authante.FeegrantKeeper,
	signModeHandler *txsigning.HandlerMap,
	ibcKeeper *ibckeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
	feeMarketKeeper feemarketkeeper.Keeper,
) sdk.AnteHandler {
	evmOptions := evmante.HandlerOptions{
		AccountKeeper:          accountKeeper,
		BankKeeper:             bankKeeper,
		FeegrantKeeper:         feegrantKeeper,
		IBCKeeper:              ibcKeeper,
		EvmKeeper:              evmKeeper,
		FeeMarketKeeper:        feeMarketKeeper,
		SignModeHandler:        signModeHandler,
		SigGasConsumer:         evmante.DefaultSigVerificationGasConsumer,
		MaxTxGasWanted:         100000,
		ExtensionOptionChecker: ethermint.HasDynamicFeeExtensionOption,
		DynamicFeeChecker:      true,
		DisabledAuthzMsgs: []string{
			sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
			sdk.MsgTypeURL(&vestingtypes.MsgCreateVestingAccount{}),
			sdk.MsgTypeURL(&vestingtypes.MsgCreatePermanentLockedAccount{}),
			sdk.MsgTypeURL(&vestingtypes.MsgCreatePeriodicVestingAccount{}),
		},
		ExtraDecorators: []sdk.AnteDecorator{},
	}

	if err := evmOptions.Validate(); err != nil {
		panic(err)
	}

	return evmante.NewEthAnteHandler(evmOptions)
}

// CheckEVMExtensionOption checks if the transaction has EVM extension options and returns the appropriate ante handler
func CheckEVMExtensionOption(tx sdk.Tx, evmAnteHandler sdk.AnteHandler) (sdk.AnteHandler, error) {
	txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
	if ok {
		opts := txWithExtensions.GetExtensionOptions()
		if len(opts) > 0 {
			switch typeURL := opts[0].GetTypeUrl(); typeURL {
			case "/ethermint.evm.v1.ExtensionOptionsEthereumTx":
				return evmAnteHandler, nil
			default:
				return nil, errorsmod.Wrapf(
					errortypes.ErrUnknownExtensionOptions,
					"rejecting tx with unsupported extension option: %s", typeURL,
				)
			}
		}
	}
	return nil, nil
}
