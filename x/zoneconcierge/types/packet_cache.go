package types

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/cosmos/cosmos-sdk/codec"
)

// MarshaledPacketData holds both the original packet data and its marshaled form
type MarshaledPacketData struct {
	PacketData    *OutboundPacket
	MarshaledData []byte
	DataSize      int
}

// PacketMarshalCache provides caching for marshaled packet data to avoid
// redundant marshaling operations when sending same headers to multiple BSNs
type PacketMarshalCache struct {
	cache map[string]*MarshaledPacketData
	cdc   codec.BinaryCodec
}

// NewPacketMarshalCache creates a new packet marshal cache
func NewPacketMarshalCache(cdc codec.BinaryCodec) *PacketMarshalCache {
	return &PacketMarshalCache{
		cache: make(map[string]*MarshaledPacketData),
		cdc:   cdc,
	}
}

// GetOrMarshal returns the marshaled data for the packet, either from cache or by marshaling
func (cache *PacketMarshalCache) GetOrMarshal(packetData *OutboundPacket) *MarshaledPacketData {
	// Only cache BTC headers packets, not other packet types like BTC timestamp
	if btcHeaders := packetData.GetBtcHeaders(); btcHeaders != nil {
		// Generate cache key based on packet content
		key := cache.generateCacheKey(packetData)

		// Check if already cached
		if cached, found := cache.cache[key]; found {
			return cached
		}

		// Marshal the data
		marshaledData := cache.cdc.MustMarshal(packetData)

		// Store in cache
		cached := &MarshaledPacketData{
			PacketData:    packetData,
			MarshaledData: marshaledData,
			DataSize:      len(marshaledData),
		}
		cache.cache[key] = cached

		return cached
	}

	// For non-BTC headers packets, just marshal without caching
	marshaledData := cache.cdc.MustMarshal(packetData)
	return &MarshaledPacketData{
		PacketData:    packetData,
		MarshaledData: marshaledData,
		DataSize:      len(marshaledData),
	}
}

// generateCacheKey creates a deterministic cache key for the packet data
// This ensures that identical packet contents get the same cache key
// this cache key generation is not significantly cheaper than full marshaling, but
// cached marshaling provides dramatic performance improvements
func (cache *PacketMarshalCache) generateCacheKey(packetData *OutboundPacket) string {
	// For BTC headers, we can create a key based on the header hashes
	// This is more efficient than marshaling the entire packet just for the key
	hasher := sha256.New()

	// Only handle BTC headers packet data for caching
	if btcHeaders := packetData.GetBtcHeaders(); btcHeaders != nil {
		hasher.Write([]byte("btc_headers:"))
		for _, header := range btcHeaders.Headers {
			if header != nil && header.Hash != nil {
				hasher.Write(header.Hash.MustMarshal())
			}
		}
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// Clear clears the cache - useful for testing or memory management
func (cache *PacketMarshalCache) Clear() {
	cache.cache = make(map[string]*MarshaledPacketData)
}

// Size returns the number of cached entries
func (cache *PacketMarshalCache) Size() int {
	return len(cache.cache)
}
