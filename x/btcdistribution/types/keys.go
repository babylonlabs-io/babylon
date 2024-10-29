package types

const (
	// ModuleName defines the module name
	ModuleName = "btcdistribution"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

var (
	DelegatorRewardsKey = []byte{0x01} // key prefix for the delegator outstanding available rewards to withdraw
)
