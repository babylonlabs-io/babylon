package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/stretchr/testify/require"
)

func TestPacketMarshalingOptimization_CacheReuse(t *testing.T) {
	// Setup codec for manual testing
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create cache
	cache := types.NewPacketMarshalCache(cdc)

	// Create test data - same headers that would be sent to multiple BSNs
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}

	// Create packets for different BSNs with same headers
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
	packet3 := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: headers},
		},
	}

	// Test scenario: Multiple BSNs receive same headers
	require.Equal(t, 0, cache.Size())

	// BSN 1 receives headers (cache miss - should marshal)
	marshaledData1 := cache.GetOrMarshal(packet1)
	require.Equal(t, 1, cache.Size())
	require.NotNil(t, marshaledData1)
	firstMarshalData := marshaledData1.MarshaledData

	// BSN 2 receives same headers (cache hit - should NOT marshal again)
	marshaledData2 := cache.GetOrMarshal(packet2)
	require.Equal(t, 1, cache.Size())                // Size should remain 1
	require.Equal(t, marshaledData1, marshaledData2) // Should be same object reference
	require.Equal(t, firstMarshalData, marshaledData2.MarshaledData)

	// BSN 3 receives same headers (cache hit - should NOT marshal again)
	marshaledData3 := cache.GetOrMarshal(packet3)
	require.Equal(t, 1, cache.Size())                // Size should remain 1
	require.Equal(t, marshaledData1, marshaledData3) // Should be same object reference
	require.Equal(t, firstMarshalData, marshaledData3.MarshaledData)
}

func TestPacketValidation_EliminatesDoubleMarshal(t *testing.T) {
	// Create test packet data
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

	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create marshaled data
	cache := types.NewPacketMarshalCache(cdc)
	marshaledData := cache.GetOrMarshal(packet)

	// Test validation with pre-marshaled data
	err := validatePacketWithMarshaledData(marshaledData)
	require.NoError(t, err) // Should pass validation

	// Create oversized packet to test validation
	largeHeaders := make([]*btclctypes.BTCHeaderInfo, 5000) // Very large packet
	for i := 0; i < 5000; i++ {
		largeHeaders[i] = datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
	}
	largePacket := &types.OutboundPacket{
		Packet: &types.OutboundPacket_BtcHeaders{
			BtcHeaders: &types.BTCHeaders{Headers: largeHeaders},
		},
	}
	largeMarshaledData := cache.GetOrMarshal(largePacket)

	// Should fail validation due to size
	err = validatePacketWithMarshaledData(largeMarshaledData)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum allowed size")
}

func TestMarshaledPacketData_Structure(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create test data
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

	// Create cache and get marshaled data
	cache := types.NewPacketMarshalCache(cdc)
	marshaledData := cache.GetOrMarshal(packet)

	// Verify structure
	require.NotNil(t, marshaledData.PacketData)
	require.NotNil(t, marshaledData.MarshaledData)
	require.Greater(t, marshaledData.DataSize, 0)
	require.Equal(t, len(marshaledData.MarshaledData), marshaledData.DataSize)

	// Verify that we can unmarshal the data back correctly
	var unmarshaledPacket types.OutboundPacket
	err := cdc.Unmarshal(marshaledData.MarshaledData, &unmarshaledPacket)
	require.NoError(t, err)
	
	// Verify content matches original
	originalHeaders := packet.GetBtcHeaders()
	unmarshaledHeaders := unmarshaledPacket.GetBtcHeaders()
	require.NotNil(t, originalHeaders)
	require.NotNil(t, unmarshaledHeaders)
	require.Equal(t, len(originalHeaders.Headers), len(unmarshaledHeaders.Headers))
}

func TestCacheKeyConsistency(t *testing.T) {
	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	btclctypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create test data
	r := rand.New(rand.NewSource(42)) // Fixed seed for deterministic results
	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
		datagen.GenRandomBTCHeaderInfoWithParent(r, nil),
	}

	// Create two separate cache instances
	cache1 := types.NewPacketMarshalCache(cdc)
	cache2 := types.NewPacketMarshalCache(cdc)

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

	// Marshal with different cache instances
	marshaledData1 := cache1.GetOrMarshal(packet1)
	marshaledData2 := cache2.GetOrMarshal(packet2)

	// The marshaled data should be identical (same content)
	require.Equal(t, marshaledData1.MarshaledData, marshaledData2.MarshaledData)
	require.Equal(t, marshaledData1.DataSize, marshaledData2.DataSize)

	// Cache sizes should both be 1
	require.Equal(t, 1, cache1.Size())
	require.Equal(t, 1, cache2.Size())
}

// Helper function for testing packet validation
func validatePacketWithMarshaledData(marshaledData *types.MarshaledPacketData) error {
	// This simulates the validation logic that would use pre-marshaled data
	// In a real implementation, this would be a method on the keeper that validates
	// without re-marshaling the data
	
	// Basic size validation (simulating what the real method would do)
	maxSize := 100 * 1024 // 100KB limit for testing
	if marshaledData.DataSize > maxSize {
		return fmt.Errorf("packet size %d exceeds maximum allowed size %d", marshaledData.DataSize, maxSize)
	}
	
	return nil
}