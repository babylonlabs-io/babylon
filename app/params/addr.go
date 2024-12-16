package params

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

var (
	AccGov          = authtypes.NewModuleAddress(govtypes.ModuleName)
	AccFeeCollector = authtypes.NewModuleAddress(authtypes.FeeCollectorName)
)
