package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// BenchmarkMarshalingWithoutCache measures the performance of marshaling without cache
func BenchmarkMarshalingWithoutCache(b *testing.B) {
	// Setup
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create test data - simulate typical BTC headers (5 headers per broadcast)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}
	packet := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	// Benchmark without cache - simulates sending same headers to multiple BSNs
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate what happens without cache: marshal twice per packet
		_ = cdc.MustMarshal(packet) // Validation marshal
		_ = cdc.MustMarshal(packet) // Send marshal
	}
}

// BenchmarkMarshalingWithCache measures the performance of marshaling with cache
func BenchmarkMarshalingWithCache(b *testing.B) {
	// Setup
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create test data
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}
	packet := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Benchmark with cache - simulates sending same headers to multiple BSNs
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// First call will cache, subsequent calls will hit cache
		_ = cache.GetOrMarshal(packet)
	}
}

// BenchmarkMultipleBSNsWithoutCache simulates broadcast to multiple BSNs without cache
func BenchmarkMultipleBSNsWithoutCache(b *testing.B) {
	// Setup
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create test data
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}
	packet := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	const numBSNs = 10 // Simulate 10 BSNs receiving same headers

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate sending to 10 BSNs without cache
		for j := 0; j < numBSNs; j++ {
			_ = cdc.MustMarshal(packet) // Validation marshal
			_ = cdc.MustMarshal(packet) // Send marshal
		}
	}
}

// BenchmarkMultipleBSNsWithCache simulates broadcast to multiple BSNs with cache
func BenchmarkMultipleBSNsWithCache(b *testing.B) {
	// Setup
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create test data
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}
	packet := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	const numBSNs = 10 // Simulate 10 BSNs receiving same headers

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create fresh cache for each iteration to simulate real usage
		cache := types.NewPacketMarshalCache(cdc)
		
		// Simulate sending to 10 BSNs with cache
		for j := 0; j < numBSNs; j++ {
			_ = cache.GetOrMarshal(packet) // First call caches, rest hit cache
		}
	}
}

// BenchmarkCacheKeyGeneration benchmarks the cache key generation
func BenchmarkCacheKeyGeneration(b *testing.B) {
	// Setup
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create test data with various sizes
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	b.Run("SmallPacket_3Headers", func(b *testing.B) {
		headers := []*btclctypes.BTCHeaderInfo{
			datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
			datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
			datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		}
		packet := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{
				BtcHeaders: &types.BTCHeaders{Headers: headers},
			},
		}
		cache := types.NewPacketMarshalCache(cdc)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			cache.Clear() // Clear cache to force key generation each time
			_ = cache.GetOrMarshal(packet)
		}
	})
	
	b.Run("LargePacket_20Headers", func(b *testing.B) {
		headers := make([]*btclctypes.BTCHeaderInfo, 20)
		for i := 0; i < 20; i++ {
			headers[i] = datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
		}
		packet := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcHeaders{
				BtcHeaders: &types.BTCHeaders{Headers: headers},
			},
		}
		cache := types.NewPacketMarshalCache(cdc)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			cache.Clear() // Clear cache to force key generation each time
			_ = cache.GetOrMarshal(packet)
		}
	})
}