package params

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	dstrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

var (
	AccGov          = authtypes.NewModuleAddress(govtypes.ModuleName)
	AccDistribution = authtypes.NewModuleAddress(dstrtypes.ModuleName)
	AccFeeCollector = authtypes.NewModuleAddress(authtypes.FeeCollectorName)
)
