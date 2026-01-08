package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "btcstaking"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_btcstaking"
)

var (
	ParamsKey           = []byte{0x01} // key prefix for the parameters
	FinalityProviderKey = []byte{0x02} // key prefix for the finality providers
	BTCDelegatorKey     = []byte{0x03} // key prefix for the BTC delegators
	BTCDelegationKey    = []byte{0x04} // key prefix for the BTC delegations
	// 0x05 was used for something else in the past
	BTCHeightKey = []byte{0x06} // key prefix for the BTC heights
	// 0x07 was used for something else in the past
	PowerDistUpdateKey        = []byte{0x08}              // key prefix for power distribution update events
	AllowedStakingTxHashesKey = collections.NewPrefix(9)  // key prefix for allowed staking tx hashes
	HeightToVersionMapKey     = collections.NewPrefix(10) // key prefix for height to version map
	LargestBtcReorgInBlocks   = collections.NewPrefix(11) // key prefix for the BTC block height difference of the largest reorg
	// prefix {12} reserved to BTCStakingEventKey
	// prefix {13} is cleaned in v3rc3 and can be reused after that upgrade is run on testnet
	// prefix {14} reserved to FinalityProviderBsnIndexKey
	// prefix {15} reserved to AllowedMultiStakingTxHashesKey
	FpBbnAddrKey             = collections.NewPrefix(16) // key prefix for index fpBbnAddr
	FinalityProvidersDeleted = collections.NewPrefix(17) // key prefix for the deleted finality provider btcPk
)
