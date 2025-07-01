package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmmonoante "github.com/cosmos/evm/ante/evm"
)

func newMonoEVMAnteHandler(
	options EVMHandlerOptions,
) sdk.AnteHandler {
	return sdk.ChainAnteDecorators(
		evmmonoante.NewEVMMonoDecorator(
			options.AccountKeeper,
			options.FeeMarketKeeper,
			options.EvmKeeper,
			options.MaxTxGasWanted,
		),
	)
}
