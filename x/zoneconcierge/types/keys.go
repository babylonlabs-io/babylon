package types

import (
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "zc"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_zc"

	// TStoreKey defines the transient store key for tracking BTC header and consumer event broadcasting triggers
	TStoreKey = "transient_zc"

	// Version defines the current version the IBC module supports
	Version = "zoneconcierge-1"

	// Ordering defines the ordering the IBC module supports
	Ordering = channeltypes.ORDERED

	// PortID is the default port id that module binds to
	PortID = "zoneconcierge"
)

// Transient store key prefixes
const (
	prefixBTCHeaderInsertedKey = iota + 100
	prefixBTCReorgOccurredKey
	prefixConsumerChannelKey
)

// KVStore keys
var (
	PortKey                  = []byte{0x01} // PortKey defines the key to store the port ID (collections.Item[string])
	LatestEpochHeadersKey    = []byte{0x02} // LatestEpochHeadersKey defines the prefix for latest headers per consumer (collections.Map[string, IndexedHeader])
	FinalizedEpochHeadersKey = []byte{0x03} // FinalizedEpochHeadersKey defines the prefix for finalized headers per consumer and epoch (collections.Map[collections.Pair[uint64, string], IndexedHeaderWithProof])
	ParamsKey                = []byte{0x04} // ParamsKey stores module parameters (collections.Item[Params])
	SealedEpochProofKey      = []byte{0x05} // SealedEpochProofKey stores proof of sealed epochs per epoch number (collections.Map[uint64, ProofEpochSealed])
	BSNBTCStateKey           = []byte{0x06} // BSNBTCStateKey stores unified BSN BTC state per consumer ID (collections.Map[string, BSNBTCState])
)

// Transient store keys
var (
	// BTCHeaderInsertedKey marks that new BTC header(s) were added in this block
	BTCHeaderInsertedKey = []byte{prefixBTCHeaderInsertedKey}
	// BTCReorgOccurredKey marks that a BTC reorg occurred in this block
	BTCReorgOccurredKey = []byte{prefixBTCReorgOccurredKey}
	// NewConsumerChannelKey marks that a new consumer channel was opened in this block
	NewConsumerChannelKey = []byte{prefixConsumerChannelKey}
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
