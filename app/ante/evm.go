package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
)

func newMonoEVMAnteHandler(
	accountKeeper AccountKeeper,
	feemarketKeeper feemarketkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
	maxTxGasWanted uint64,
	preciseBankKeeper precisebankkeeper.Keeper,
) sdk.AnteHandler {
	return sdk.ChainAnteDecorators(
		NewEVMMonoDecorator(
			accountKeeper,
			feemarketKeeper,
			evmKeeper,
			maxTxGasWanted,
			preciseBankKeeper,
		),
	)
}
