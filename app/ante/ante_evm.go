package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmante "github.com/cosmos/evm/ante/evm"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
)

// newMonoEVMAnteHandler creates the sdk.AnteHandler implementation for the EVM transactions.
func newMonoEVMAnteHandler(
	accountKeeper AccountKeeper,
	feemarketKeeper feemarketkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
	maxTxGasWanted uint64,
) sdk.AnteHandler {
	return sdk.ChainAnteDecorators(
		evmante.NewEVMMonoDecorator(
			accountKeeper,
			feemarketKeeper,
			evmKeeper,
			maxTxGasWanted,
		),
	)
}
