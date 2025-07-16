package params

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	dstrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	btcstktypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

var (
	AccGov                       = authtypes.NewModuleAddress(govtypes.ModuleName)
	AccDistribution              = authtypes.NewModuleAddress(dstrtypes.ModuleName)
	AccFeeCollector              = authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	AccFinality                  = authtypes.NewModuleAddress(finalitytypes.ModuleName)
	AccBTCStaking                = authtypes.NewModuleAddress(btcstktypes.ModuleName)
	ModAccCommissionCollectorBSN = "commission_collector_bsn"
)
