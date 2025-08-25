package types

import (
	"cosmossdk.io/collections"
)

const (
	// ModuleName defines the module name
	ModuleName = "coostaking"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

var (
	ParamsKey                        = collections.NewPrefix(1) // key prefix for the parameters
	HistoricalRewardsKeyPrefix       = collections.NewPrefix(2) // key prefix for (period) => HistoricalRewards
	CurrentRewardsKeyPrefix          = collections.NewPrefix(3) // key prefix for CurrentRewards
	CoostakerRewardsTrackerKeyPrefix = collections.NewPrefix(4) // key prefix for (coostaker_addr) => CoostakerRewardsTracker
)
