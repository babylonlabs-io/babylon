package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "incentive"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_incentive"
)

var (
	ParamsKey                         = []byte{0x01}             // key prefix for the parameters
	BTCStakingGaugeKey                = []byte{0x02}             // key prefix for BTC staking gauge at each height
	ReservedKey                       = []byte{0x03}             // reserved //nolint:unused
	RewardGaugeKey                    = []byte{0x04}             // key prefix for reward gauge for a given stakeholder in a given type
	RefundableMsgKeySetPrefix         = collections.NewPrefix(5) // key prefix for refundable msg key set
	FinalityProviderCurrentRewardsKey = []byte{0x06}             // key prefix for storing the Current rewards of finality provider by addr
)
