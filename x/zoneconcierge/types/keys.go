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

	// Version defines the current version the IBC module supports
	Version = "zoneconcierge-1"

	// Ordering defines the ordering the IBC module supports
	Ordering = channeltypes.ORDERED

	// PortID is the default port id that module binds to
	PortID = "zoneconcierge"
)

var (
	PortKey                  = []byte{0x11} // PortKey defines the key to store the port ID (collections.Item[string])
	LatestEpochHeadersKey    = []byte{0x12} // LatestEpochHeadersKey defines the prefix for latest headers per consumer (collections.Map[string, IndexedHeader])
	FinalizedEpochHeadersKey = []byte{0x13} // FinalizedEpochHeadersKey defines the prefix for finalized headers per consumer and epoch (collections.Map[collections.Pair[uint64, string], IndexedHeaderWithProof])
	LastSentBTCSegmentKey    = []byte{0x14} // LastSentBTCSegmentKey stores the last sent BTC segment (collections.Item[BTCChainSegment])
	ParamsKey                = []byte{0x15} // ParamsKey stores module parameters (collections.Item[Params])
	SealedEpochProofKey      = []byte{0x16} // SealedEpochProofKey stores proof of sealed epochs per epoch number (collections.Map[uint64, ProofEpochSealed])
	BSNBTCStateKey           = []byte{0x17} // BSNBTCStateKey stores unified BSN BTC state per consumer ID (collections.Map[string, BSNBTCState])
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
