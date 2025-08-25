package types_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/stretchr/testify/require"
)

func TestPacketMarshalCache_BTCHeaders(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Generate test BTC headers
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}

	// Create identical packets
	packet1 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}
	packet2 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	// Test cache miss (first call should marshal)
	require.Equal(t, 0, cache.Size())

	marshaledData1 := cache.GetOrMarshal(packet1)
	require.Equal(t, 1, cache.Size())
	require.NotNil(t, marshaledData1)
	require.Greater(t, len(marshaledData1.MarshaledData), 0)
	require.Equal(t, len(marshaledData1.MarshaledData), marshaledData1.DataSize)

	// Test cache hit (second call should use cache)
	marshaledData2 := cache.GetOrMarshal(packet2)
	require.Equal(t, 1, cache.Size())                // Size should remain same
	require.Equal(t, marshaledData1, marshaledData2) // Should return same object

	// Test that marshaled data is correct
	require.Equal(t, marshaledData1.MarshaledData, marshaledData2.MarshaledData)

	// Verify we can unmarshal the data correctly
	var unmarshaledPacket types.OutboundPacket
	err := cdc.Unmarshal(marshaledData1.MarshaledData, &unmarshaledPacket)
	require.NoError(t, err)

	btcHeaders := unmarshaledPacket.GetBtcHeaders()
	require.NotNil(t, btcHeaders)
	require.Equal(t, len(headers), len(btcHeaders.Headers))
}

func TestPacketMarshalCache_BTCTimestamp_NotCached(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Create BTC timestamp packets with different data to ensure different objects
	timestamp1 := &types.BTCTimestamp{
		EpochInfo: &epochingtypes.Epoch{
			EpochNumber: 123,
		},
	}
	timestamp2 := &types.BTCTimestamp{
		EpochInfo: &epochingtypes.Epoch{
			EpochNumber: 124, // Different epoch number
		},
	}

	packet1 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcTimestamp{
			BtcTimestamp: timestamp1,
		},
	}
	packet2 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcTimestamp{
			BtcTimestamp: timestamp2,
		},
	}

	// BTC timestamp packets should not be cached (only BTC headers are cached)
	require.Equal(t, 0, cache.Size())

	marshaledData1 := cache.GetOrMarshal(packet1)
	require.Equal(t, 0, cache.Size()) // Should remain 0 since timestamp packets are not cached
	require.NotNil(t, marshaledData1)

	marshaledData2 := cache.GetOrMarshal(packet2)
	require.Equal(t, 0, cache.Size())                   // Should remain 0
	require.NotEqual(t, marshaledData1, marshaledData2) // Should be different objects since not cached
	// The marshaled data should be different since we're using different timestamp data
	require.NotEqual(t, marshaledData1.MarshaledData, marshaledData2.MarshaledData)
}

func TestPacketMarshalCache_DifferentPackets(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Generate different BTC headers
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers1 := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}
	headers2 := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}

	packet1 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers1},
		},
	}
	packet2 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers2},
		},
	}

	// Different packets should create different cache entries
	marshaledData1 := cache.GetOrMarshal(packet1)
	require.Equal(t, 1, cache.Size())

	marshaledData2 := cache.GetOrMarshal(packet2)
	require.Equal(t, 2, cache.Size())                   // Should have 2 entries now
	require.NotEqual(t, marshaledData1, marshaledData2) // Should be different objects
	require.NotEqual(t, marshaledData1.MarshaledData, marshaledData2.MarshaledData)
}

func TestPacketMarshalCache_ClearAndSize(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Add some entries
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}
	packet := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	cache.GetOrMarshal(packet)
	require.Equal(t, 1, cache.Size())

	// Test clear
	cache.Clear()
	require.Equal(t, 0, cache.Size())

	// Add again after clear
	cache.GetOrMarshal(packet)
	require.Equal(t, 1, cache.Size())
}

func TestPacketMarshalCache_CacheKeyGeneration(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Create packets with same headers in different order
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	header1 := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
	header2 := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)

	packet1 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{
				Headers: []*btclctypes.BTCHeaderInfo{header1, header2},
			},
		},
	}
	packet2 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{
				Headers: []*btclctypes.BTCHeaderInfo{header1, header2}, // Same order
			},
		},
	}
	packet3 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{
				Headers: []*btclctypes.BTCHeaderInfo{header2, header1}, // Different order
			},
		},
	}

	// Same headers in same order should hit cache
	marshaledData1 := cache.GetOrMarshal(packet1)
	require.Equal(t, 1, cache.Size())

	marshaledData2 := cache.GetOrMarshal(packet2)
	require.Equal(t, 1, cache.Size()) // Should be cache hit
	require.Equal(t, marshaledData1, marshaledData2)

	// Different order should create new cache entry
	marshaledData3 := cache.GetOrMarshal(packet3)
	require.Equal(t, 2, cache.Size()) // Should be cache miss
	require.NotEqual(t, marshaledData1, marshaledData3)
}

// TestPacketCacheWithEdgeCases validates packet caching behavior on edge cases
func TestPacketCacheWithEdgeCases(t *testing.T) {
	// Setup test environment
	babylonApp := app.Setup(t, false)
	cdc := babylonApp.AppCodec()
	cache := types.NewPacketMarshalCache(cdc)

	// Test empty headers array and nil header handling
	t.Run("BTCHeaders_Empty_Headers_Handling", func(t *testing.T) {
		cache.Clear()

		// Create packet with empty headers
		btcHeaders := &types.BTCHeaders{Headers: []*btclctypes.BTCHeaderInfo{}}
		packetData := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{BtcHeaders: btcHeaders},
		}

		// Verify it still gets cached (empty headers are valid)
		result := cache.GetOrMarshal(packetData)
		require.NotNil(t, result, "Empty headers packet should still be processed")
		require.Equal(t, 1, cache.Size(), "Empty headers packet should be cached")
		require.Greater(t, result.DataSize, 0, "Even empty headers should have some marshaled size")

		// Verify GetBtcHeaders works with empty headers
		retrievedHeaders := packetData.GetBtcHeaders()
		require.NotNil(t, retrievedHeaders, "GetBtcHeaders should return non-nil even for empty headers")
		require.Equal(t, 0, len(retrievedHeaders.Headers), "Empty headers should have 0 length")
	})

	t.Run("BTCHeaders_Safe_Marshaling_Without_Nil_Headers", func(t *testing.T) {
		cache.Clear()

		// Create packet with valid headers (avoid nil which causes marshaling panic)
		headers := make([]*btclctypes.BTCHeaderInfo, 3)
		headers[0] = datagen.GenRandomBTCHeaderInfo(rand.New(rand.NewSource(333)))
		headers[1] = datagen.GenRandomBTCHeaderInfo(rand.New(rand.NewSource(444)))
		headers[2] = datagen.GenRandomBTCHeaderInfo(rand.New(rand.NewSource(555)))

		btcHeaders := &types.BTCHeaders{Headers: headers}
		packetData := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{BtcHeaders: btcHeaders},
		}

		// Should work and cache the packet
		result := cache.GetOrMarshal(packetData)
		require.NotNil(t, result, "Valid packet should be processed")
		require.Equal(t, 1, cache.Size(), "Valid packet should be cached")
		require.Greater(t, result.DataSize, 0, "Should have marshaled data size > 0")

		// Verify the packet structure is maintained
		retrievedHeaders := packetData.GetBtcHeaders()
		require.NotNil(t, retrievedHeaders, "GetBtcHeaders should return non-nil")
		require.Equal(t, 3, len(retrievedHeaders.Headers), "Should maintain original array length")
		require.NotNil(t, retrievedHeaders.Headers[0], "First header should not be nil")
		require.NotNil(t, retrievedHeaders.Headers[1], "Second header should not be nil")
		require.NotNil(t, retrievedHeaders.Headers[2], "Third header should not be nil")

		// Test cache hit with same packet
		result2 := cache.GetOrMarshal(packetData)
		require.Same(t, result, result2, "Should return same cached result")
	})

	t.Run("BTCTimestamp_Packet_No_Caching", func(t *testing.T) {
		cache.Clear()

		// Create BTCTimestamp packet (not BTC headers)
		indexedHeader := &types.IndexedHeader{
			ConsumerId: "test-consumer",
			Hash:       []byte("test-hash"),
			Height:     100,
		}

		btcTimestamp := &types.BTCTimestamp{
			Header: indexedHeader,
		}

		packetData := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcTimestamp{
				BtcTimestamp: btcTimestamp,
			},
		}

		// Verify GetBtcHeaders returns nil for non-BTC headers packet
		retrievedHeaders := packetData.GetBtcHeaders()
		require.Nil(t, retrievedHeaders, "GetBtcHeaders should return nil for non-BTC headers packet")

		// Test that it's not cached
		require.Equal(t, 0, cache.Size(), "Cache should be empty initially")

		result1 := cache.GetOrMarshal(packetData)
		require.NotNil(t, result1, "GetOrMarshal should return result even for non-BTC headers")
		require.Equal(t, 0, cache.Size(), "Cache should remain empty for non-BTC headers packets")

		result2 := cache.GetOrMarshal(packetData)
		require.NotNil(t, result2, "Second call should also return result")
		require.Equal(t, 0, cache.Size(), "Cache should still be empty")

		// Results should be different objects (no caching)
		require.NotSame(t, result1, result2, "Results should be different objects (not cached)")

		// But marshaled data should be identical (same content)
		require.Equal(t, result1.MarshaledData, result2.MarshaledData, "Marshaled data should be identical")
	})

	t.Run("NonBTCHeaders_Packet_No_Caching_Verification", func(t *testing.T) {
		cache.Clear()

		// Create another BTCTimestamp packet to verify non-caching behavior
		indexedHeader2 := &types.IndexedHeader{
			ConsumerId: "test-consumer-2",
			Hash:       []byte("different-hash"),
			Height:     200,
		}

		btcTimestamp2 := &types.BTCTimestamp{
			Header: indexedHeader2,
		}

		packetData2 := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcTimestamp{
				BtcTimestamp: btcTimestamp2,
			},
		}

		// Verify GetBtcHeaders returns nil for this packet too
		retrievedHeaders := packetData2.GetBtcHeaders()
		require.Nil(t, retrievedHeaders, "GetBtcHeaders should return nil for BTCTimestamp packet")

		// Test that it's not cached (cache should remain empty)
		result1 := cache.GetOrMarshal(packetData2)
		require.NotNil(t, result1, "GetOrMarshal should return result for BTCTimestamp packet")
		require.Equal(t, 0, cache.Size(), "Cache should remain empty for non-BTC headers packets")

		result2 := cache.GetOrMarshal(packetData2)
		require.NotSame(t, result1, result2, "Results should be different objects (not cached)")
		require.Equal(t, result1.MarshaledData, result2.MarshaledData, "Marshaled data should be identical for same packet")
	})
}

// FuzzTestPacketMarshalCache_CacheKeyConsistency tests that:
// 1. Identical packets produce identical cache keys
// 2. Different packets produce different cache keys
func FuzzTestPacketMarshalCache_CacheKeyConsistency(f *testing.F) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Add seed test cases
	f.Add(uint64(1), uint64(1), uint64(100), uint64(100)) // Same headers
	f.Add(uint64(1), uint64(2), uint64(100), uint64(200)) // Different headers
	f.Add(uint64(1), uint64(1), uint64(100), uint64(200)) // Same first, different second

	f.Fuzz(func(t *testing.T, height1, height2, work1, work2 uint64) {
		// Create deterministic headers using the fuzz inputs
		// We'll create headers with specific values to ensure predictable behavior
		r := rand.New(rand.NewSource(12345)) // Fixed seed for consistent test behavior

		// Generate base headers with deterministic content
		header1 := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
		header1.Height = uint32(height1 % 1000000) // Limit height to avoid overflow
		workUint1 := math.NewUint(work1 % 1000000) // Limit work to avoid overflow
		header1.Work = &workUint1                  // Set work directly

		header2 := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
		header2.Height = uint32(height2 % 1000000) // Limit height to avoid overflow 
		workUint2 := math.NewUint(work2 % 1000000) // Limit work to avoid overflow
		header2.Work = &workUint2                  // Set work directly

		// Test 1: Same headers in same order should be cached identically
		packet1a := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{
				BtcHeaders: &types.BTCHeaders{
					Headers: []*btclctypes.BTCHeaderInfo{header1, header2},
				},
			},
		}

		packet1b := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{
				BtcHeaders: &types.BTCHeaders{
					Headers: []*btclctypes.BTCHeaderInfo{header1, header2}, // Same headers, same order
				},
			},
		}

		marshaledData1a := cache.GetOrMarshal(packet1a)
		marshaledData1b := cache.GetOrMarshal(packet1b)

		// Since packets contain identical header references, should get same cached object
		require.Equal(t, marshaledData1a, marshaledData1b, "Identical packet references should return same cached object")
		require.Equal(t, 1, cache.Size(), "Should have only one cached entry for identical packet references")

		// Test 2: Different order should create separate cache entry
		packet2 := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{
				BtcHeaders: &types.BTCHeaders{
					Headers: []*btclctypes.BTCHeaderInfo{header2, header1}, // Different order
				},
			},
		}

		marshaledData2 := cache.GetOrMarshal(packet2)

		// Different order should create different cache entry
		// Since we use different random seeds for each header, they should be different
		require.NotEqual(t, marshaledData1a, marshaledData2, "Different packet order should create separate cache entry")
		require.Equal(t, 2, cache.Size(), "Should have two cache entries for different packet orders")

		// Test 3: Completely different headers should create separate entry
		header3 := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
		header3.Height = uint32((height1 + height2 + 1) % 1000000) // Ensure different
		workUint3 := math.NewUint((work1 + work2 + 1) % 1000000)   // Ensure different
		header3.Work = &workUint3

		packet3 := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{
				BtcHeaders: &types.BTCHeaders{
					Headers: []*btclctypes.BTCHeaderInfo{header3},
				},
			},
		}

		initialCacheSize := cache.Size()
		marshaledData3 := cache.GetOrMarshal(packet3)

		require.NotEqual(t, marshaledData1a, marshaledData3, "Different headers should create separate cache entry")
		require.Equal(t, initialCacheSize+1, cache.Size(), "Cache size should increase for different packet")

		// Clear cache for next iteration
		cache.Clear()
	})
}

// TestPacketMarshalCache_CacheKeyDeterminism tests that cache keys are deterministic
func TestPacketMarshalCache_CacheKeyDeterminism(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Create same packet multiple times with same headers
	r := rand.New(rand.NewSource(42)) // Fixed seed for deterministic headers
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}

	// Create packets independently but with same header references
	packet1 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	packet2 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers}, // Same header references
		},
	}

	// Test caching behavior - should be identical due to same header references
	marshaledData1 := cache.GetOrMarshal(packet1)
	require.Equal(t, 1, cache.Size())

	marshaledData2 := cache.GetOrMarshal(packet2)
	require.Equal(t, 1, cache.Size())                // Should still be 1 due to cache hit
	require.Equal(t, marshaledData1, marshaledData2) // Same cached object

	// Verify the marshaled data is identical
	require.Equal(t, marshaledData1.MarshaledData, marshaledData2.MarshaledData)
	require.Equal(t, marshaledData1.DataSize, marshaledData2.DataSize)
}
