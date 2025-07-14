package types

import (
	"cosmossdk.io/collections"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "btcstaking"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_btcstaking"

	// AccCommissionCollectorBSN is the module account that collects Babylon's commission from BSN rewards
	AccCommissionCollectorBSN = "commission_collector_bsn"
)

var (
	ParamsKey           = []byte{0x01} // key prefix for the parameters
	FinalityProviderKey = []byte{0x02} // key prefix for the finality providers
	BTCDelegatorKey     = []byte{0x03} // key prefix for the BTC delegators
	BTCDelegationKey    = []byte{0x04} // key prefix for the BTC delegations
	// 0x05 was used for something else in the past
	BTCHeightKey = []byte{0x06} // key prefix for the BTC heights
	// 0x07 was used for something else in the past
	PowerDistUpdateKey          = []byte{0x08}              // key prefix for power distribution update events
	AllowedStakingTxHashesKey   = collections.NewPrefix(9)  // key prefix for allowed staking tx hashes
	HeightToVersionMapKey       = []byte{0x10}              // key prefix for height to version map
	BTCConsumerDelegatorKey     = []byte{0x11}              // key prefix for the Consumer BTC delegators
	BTCStakingEventKey          = []byte{0x12}              // key prefix for the BTC staking events
	LargestBtcReorgInBlocks     = collections.NewPrefix(13) // key prefix for the BTC block height difference of the largest reorg
	FinalityProviderBsnIndexKey = []byte{0x14}              // key prefix for the finality provider BSN index
)

func BuildBsnIndexKey(bsnId string, btcPK *bbn.BIP340PubKey) []byte {
	return append([]byte(bsnId), btcPK.MustMarshal()...)
}

func BuildBsnIndexPrefix(bsnId string) []byte {
	return []byte(bsnId)
}
