package types

import "cosmossdk.io/collections"

var (
	MinterKey = collections.NewPrefix(0)
	//nolint:unused
	reservedKey    = collections.NewPrefix(1) // reserved for parameters
	GenesisTimeKey = collections.NewPrefix(2)
)

const (
	// ModuleName is the name of the mint module.
	ModuleName = "mint"

	// StoreKey is the default store key for mint
	StoreKey = ModuleName

	// QuerierRoute is the querier route for the mint store.
	QuerierRoute = StoreKey

	// Query endpoints supported by the mint querier
	QueryInflationRate    = "inflation_rate"
	QueryAnnualProvisions = "annual_provisions"
	QueryGenesisTime      = "genesis_time"
)
