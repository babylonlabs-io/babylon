package types

import (
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "zoneconcierge"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_zoneconcierge"

	// Version defines the current version the IBC module supports
	Version = "zoneconcierge-1"

	// Ordering defines the ordering the IBC module supports
	Ordering = channeltypes.ORDERED

	// PortID is the default port id that module binds to
	PortID = "zoneconcierge"
)

var (
	PortKey                  = []byte{0x11} // PortKey defines the key to store the port ID in store
	LatestEpochHeadersKey    = []byte{0x12} // LatestEpochHeadersKey defines the key to store the latest headers for each consumer in the current epoch
	FinalizedEpochHeadersKey = []byte{0x13} // FinalizedEpochHeadersKey defines the key to store finalized headers for each consumer and epoch
	LastSentBTCSegmentKey    = []byte{0x14} // LastSentBTCSegmentKey is key holding last btc light client segment sent to other cosmos zones
	ParamsKey                = []byte{0x15} // key prefix for the parameters
	SealedEpochProofKey      = []byte{0x16} // key prefix for proof of sealed epochs
	ConsumerBTCStateKey      = []byte{0x17} // key prefix for unified Consumer BTC state
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
