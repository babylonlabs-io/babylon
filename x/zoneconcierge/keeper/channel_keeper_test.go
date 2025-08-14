package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app"
	zckeeper "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/keeper"
	zctypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

func setupChannelKeeperTest(t *testing.T) (*app.BabylonApp, sdk.Context, *zckeeper.ChannelKeeper) {
	babylonApp := app.Setup(t, false)
	ctx := babylonApp.NewContext(false)
	channelKeeper := zckeeper.NewChannelKeeper(babylonApp.IBCKeeper.ChannelKeeper)
	return babylonApp, ctx, channelKeeper
}

// Helper function to setup IBC infrastructure for consumer
func setupConsumerChannel(app *app.BabylonApp, ctx sdk.Context, consumerID, channelID string, state channeltypes.State) {
	// Set client state
	app.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID, &ibctmtypes.ClientState{})

	// Set connection
	app.IBCKeeper.ConnectionKeeper.SetConnection(
		ctx, consumerID, connectiontypes.ConnectionEnd{
			ClientId: consumerID,
		},
	)

	// Set channel with the specified state
	app.IBCKeeper.ChannelKeeper.SetChannel(
		ctx, zctypes.PortID, channelID, channeltypes.Channel{
			State:          state,
			ConnectionHops: []string{consumerID},
		},
	)
}

func TestGetChannelForConsumer(t *testing.T) {
	// Test case: Channel not found
	t.Run("channel_not_found", func(t *testing.T) {
		_, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-1"
		channelID := "channel-1"

		channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
		require.False(t, found, "Should not find non-existent channel")
		require.Equal(t, channeltypes.IdentifiedChannel{}, channel, "Should return empty IdentifiedChannel")
	})

	// Test case: Channel exists but not open (INIT state)
	t.Run("channel_not_open_init", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-2"
		channelID := "channel-2"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.INIT)

		channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
		require.False(t, found, "Should not find channel in INIT state")
		require.Equal(t, channeltypes.IdentifiedChannel{}, channel, "Should return empty IdentifiedChannel")
	})

	// Test case: Channel exists but not open (TRYOPEN state)
	t.Run("channel_not_open_tryopen", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-3"
		channelID := "channel-3"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.TRYOPEN)

		channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
		require.False(t, found, "Should not find channel in TRYOPEN state")
		require.Equal(t, channeltypes.IdentifiedChannel{}, channel, "Should return empty IdentifiedChannel")
	})

	// Test case: Channel exists but closed
	t.Run("channel_closed", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-4"
		channelID := "channel-4"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.CLOSED)

		channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
		require.False(t, found, "Should not find closed channel")
		require.Equal(t, channeltypes.IdentifiedChannel{}, channel, "Should return empty IdentifiedChannel")
	})

	// Test case: Channel is open but client ID doesn't match consumer ID
	t.Run("client_id_mismatch", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-5"
		channelID := "channel-5"
		differentClientID := "different-client-id"

		// Setup with different client ID
		babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, differentClientID, &ibctmtypes.ClientState{})
		babylonApp.IBCKeeper.ConnectionKeeper.SetConnection(
			ctx, differentClientID, connectiontypes.ConnectionEnd{
				ClientId: differentClientID,
			},
		)
		babylonApp.IBCKeeper.ChannelKeeper.SetChannel(
			ctx, zctypes.PortID, channelID, channeltypes.Channel{
				State:          channeltypes.OPEN,
				ConnectionHops: []string{differentClientID},
			},
		)

		channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
		require.False(t, found, "Should not find channel with mismatched client ID")
		require.Equal(t, channeltypes.IdentifiedChannel{}, channel, "Should return empty IdentifiedChannel")
	})

	// Test case: Valid open channel with matching client ID
	t.Run("valid_open_channel", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-6"
		channelID := "channel-6"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.OPEN)

		channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
		require.True(t, found, "Should find valid open channel")
		require.NotEqual(t, channeltypes.IdentifiedChannel{}, channel, "Should return non-empty IdentifiedChannel")

		// Verify channel details
		require.Equal(t, zctypes.PortID, channel.PortId, "Should have correct port ID")
		require.Equal(t, channelID, channel.ChannelId, "Should have correct channel ID")
		require.Equal(t, channeltypes.OPEN, channel.State, "Should be in OPEN state")
		require.Equal(t, []string{consumerID}, channel.ConnectionHops, "Should have correct connection hops")
	})

	// Test case: Multiple consumers, ensure correct matching
	t.Run("multiple_consumers_correct_matching", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumer1ID := "consumer-7"
		channel1ID := "channel-7"
		consumer2ID := "consumer-8"
		channel2ID := "channel-8"

		// Setup two different consumers
		setupConsumerChannel(babylonApp, ctx, consumer1ID, channel1ID, channeltypes.OPEN)
		setupConsumerChannel(babylonApp, ctx, consumer2ID, channel2ID, channeltypes.OPEN)

		// Test first consumer
		channel1, found1 := channelKeeper.GetChannelForConsumer(ctx, consumer1ID, channel1ID)
		require.True(t, found1, "Should find first consumer's channel")
		require.Equal(t, channel1ID, channel1.ChannelId, "Should return correct channel ID for first consumer")
		require.Equal(t, []string{consumer1ID}, channel1.ConnectionHops, "Should have correct connection for first consumer")

		// Test second consumer
		channel2, found2 := channelKeeper.GetChannelForConsumer(ctx, consumer2ID, channel2ID)
		require.True(t, found2, "Should find second consumer's channel")
		require.Equal(t, channel2ID, channel2.ChannelId, "Should return correct channel ID for second consumer")
		require.Equal(t, []string{consumer2ID}, channel2.ConnectionHops, "Should have correct connection for second consumer")

		// Test cross-consumer lookup (should fail)
		_, found_cross := channelKeeper.GetChannelForConsumer(ctx, consumer1ID, channel2ID)
		require.False(t, found_cross, "Should not find channel with mismatched consumer/channel combination")
	})

	// Test case: Same consumer ID, different channel IDs
	t.Run("same_consumer_multiple_channels", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-9"
		channel1ID := "channel-9-1"
		channel2ID := "channel-9-2"

		// Setup same consumer with multiple channels
		setupConsumerChannel(babylonApp, ctx, consumerID, channel1ID, channeltypes.OPEN)
		setupConsumerChannel(babylonApp, ctx, consumerID, channel2ID, channeltypes.OPEN)

		// Test both channels
		channel1, found1 := channelKeeper.GetChannelForConsumer(ctx, consumerID, channel1ID)
		require.True(t, found1, "Should find first channel")
		require.Equal(t, channel1ID, channel1.ChannelId, "Should return correct first channel ID")

		channel2, found2 := channelKeeper.GetChannelForConsumer(ctx, consumerID, channel2ID)
		require.True(t, found2, "Should find second channel")
		require.Equal(t, channel2ID, channel2.ChannelId, "Should return correct second channel ID")
	})

	// Test case: Test with context cancellation scenarios
	t.Run("context_handling", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-12"
		channelID := "channel-12"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.OPEN)

		// Test with a fresh context
		freshCtx := babylonApp.NewContext(false)
		channel, found := channelKeeper.GetChannelForConsumer(freshCtx, consumerID, channelID)
		require.True(t, found, "Should find channel with fresh context")
		require.Equal(t, channelID, channel.ChannelId, "Should return correct channel ID with fresh context")
	})
}

func TestConsumerHasIBCChannelOpen(t *testing.T) {
	// Test case: Channel not found
	t.Run("channel_not_found", func(t *testing.T) {
		_, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-not-exist"
		channelID := "channel-not-exist"

		hasOpen := channelKeeper.ConsumerHasIBCChannelOpen(ctx, consumerID, channelID)
		require.False(t, hasOpen, "Should return false for non-existent channel")
	})

	// Test case: Channel exists but not open
	t.Run("channel_exists_not_open", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-not-open-2"
		channelID := "channel-not-open-2"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.INIT)

		hasOpen := channelKeeper.ConsumerHasIBCChannelOpen(ctx, consumerID, channelID)
		require.False(t, hasOpen, "Should return false for channel in INIT state")
	})

	// Test case: Valid open channel
	t.Run("valid_open_channel", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-hasopen"
		channelID := "channel-hasopen"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.OPEN)

		hasOpen := channelKeeper.ConsumerHasIBCChannelOpen(ctx, consumerID, channelID)
		require.True(t, hasOpen, "Should return true for valid open channel")
	})

	// Test case: Client ID mismatch
	t.Run("client_id_mismatch", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-mismatch"
		channelID := "channel-hasopen-2"
		differentClientID := "different-client-hasopen-4"

		// Setup with different client ID
		babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, differentClientID, &ibctmtypes.ClientState{})
		babylonApp.IBCKeeper.ConnectionKeeper.SetConnection(
			ctx, differentClientID, connectiontypes.ConnectionEnd{
				ClientId: differentClientID,
			},
		)
		babylonApp.IBCKeeper.ChannelKeeper.SetChannel(
			ctx, zctypes.PortID, channelID, channeltypes.Channel{
				State:          channeltypes.OPEN,
				ConnectionHops: []string{differentClientID},
			},
		)

		hasOpen := channelKeeper.ConsumerHasIBCChannelOpen(ctx, consumerID, channelID)
		require.False(t, hasOpen, "Should return false for mismatched client ID")
	})
}

func TestGetChannelForConsumer_IdentifiedChannelStructure(t *testing.T) {
	babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)

	consumerID := "consumer-structure-test"
	channelID := "channel-structure-test"

	setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.OPEN)

	channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
	require.True(t, found, "Should find the channel")

	// Test the structure of IdentifiedChannel
	t.Run("identified_channel_structure", func(t *testing.T) {
		require.IsType(t, channeltypes.IdentifiedChannel{}, channel, "Should return IdentifiedChannel type")
		require.NotEmpty(t, channel.PortId, "Port ID should not be empty")
		require.NotEmpty(t, channel.ChannelId, "Channel ID should not be empty")
		require.Equal(t, zctypes.PortID, channel.PortId, "Should use zoneconcierge port ID")
		require.Equal(t, channelID, channel.ChannelId, "Should return the correct channel ID")
	})

	// Test the Channel field within IdentifiedChannel
	t.Run("channel_field_structure", func(t *testing.T) {
		require.Equal(t, channeltypes.OPEN, channel.State, "Channel should be in OPEN state")
		require.NotEmpty(t, channel.ConnectionHops, "Connection hops should not be empty")
		require.Contains(t, channel.ConnectionHops, consumerID, "Connection hops should contain consumer ID")
		require.Len(t, channel.ConnectionHops, 1, "Should have exactly one connection hop")
	})
}

func TestGetChannelForConsumer_AdditionalEdgeCases(t *testing.T) {
	babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)

	// Test with different channel states
	t.Run("all_channel_states", func(t *testing.T) {
		consumerID := "consumer-states"

		// Test all possible channel states
		states := []struct {
			state    channeltypes.State
			expected bool
			name     string
		}{
			{channeltypes.UNINITIALIZED, false, "UNINITIALIZED"},
			{channeltypes.INIT, false, "INIT"},
			{channeltypes.TRYOPEN, false, "TRYOPEN"},
			{channeltypes.OPEN, true, "OPEN"},
			{channeltypes.CLOSED, false, "CLOSED"},
		}

		for i, test := range states {
			channelID := fmt.Sprintf("channel-state-%d", i)
			setupConsumerChannel(babylonApp, ctx, consumerID, channelID, test.state)

			_, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
			if test.expected {
				require.True(t, found, "Should find channel in %s state", test.name)
			} else {
				require.False(t, found, "Should not find channel in %s state", test.name)
			}
		}
	})

	// Test multiple sequential calls for consistency
	t.Run("consistency_check", func(t *testing.T) {
		consumerID := "consumer-consistency"
		channelID := "channel-consistency"

		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.OPEN)

		// Call multiple times to ensure consistency
		for i := 0; i < 5; i++ {
			channel, found := channelKeeper.GetChannelForConsumer(ctx, consumerID, channelID)
			require.True(t, found, "Call %d should succeed", i)
			require.Equal(t, channelID, channel.ChannelId, "Channel ID should be consistent on call %d", i)
			require.Equal(t, channeltypes.OPEN, channel.State, "Channel state should be consistent on call %d", i)
		}
	})
}

func TestGetAllOpenZCChannels(t *testing.T) {
	babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)

	// Test empty case - no channels exist
	t.Run("no_channels_exist", func(t *testing.T) {
		openChannels := channelKeeper.GetAllOpenZCChannels(ctx)
		require.Empty(t, openChannels, "Should return empty slice when no channels exist")
		require.NotNil(t, openChannels, "Should return non-nil empty slice")
	})

	// Test single open channel
	t.Run("single_open_channel", func(t *testing.T) {
		// Create fresh context to avoid interference
		ctx1 := babylonApp.NewContext(false)

		consumerID := "consumer-single"
		channelID := "channel-single"

		setupConsumerChannel(babylonApp, ctx1, consumerID, channelID, channeltypes.OPEN)

		openChannels := channelKeeper.GetAllOpenZCChannels(ctx1)
		require.Len(t, openChannels, 1, "Should return exactly one open channel")
		require.Equal(t, channelID, openChannels[0].ChannelId, "Should return the correct channel")
		require.Equal(t, zctypes.PortID, openChannels[0].PortId, "Should use zoneconcierge port")
		require.Equal(t, channeltypes.OPEN, openChannels[0].State, "Should be in OPEN state")
	})

	// Test channels in different states - only OPEN should be returned
	t.Run("filter_by_open_state", func(t *testing.T) {
		babylonApp, ctx, channelKeeper = setupChannelKeeperTest(t)

		// Setup channels in different states
		states := []struct {
			consumerID string
			channelID  string
			state      channeltypes.State
			name       string
		}{
			{"consumer-uninit", "channel-uninit", channeltypes.UNINITIALIZED, "UNINITIALIZED"},
			{"consumer-init", "channel-init", channeltypes.INIT, "INIT"},
			{"consumer-tryopen", "channel-tryopen", channeltypes.TRYOPEN, "TRYOPEN"},
			{"consumer-open1", "channel-open1", channeltypes.OPEN, "OPEN1"},
			{"consumer-open2", "channel-open2", channeltypes.OPEN, "OPEN2"},
			{"consumer-closed", "channel-closed", channeltypes.CLOSED, "CLOSED"},
		}

		for _, test := range states {
			setupConsumerChannel(babylonApp, ctx, test.consumerID, test.channelID, test.state)
		}

		openChannels := channelKeeper.GetAllOpenZCChannels(ctx)
		require.Len(t, openChannels, 2, "Should return only OPEN channels")

		// Verify all returned channels are OPEN
		openChannelIDs := make(map[string]bool)
		for _, channel := range openChannels {
			require.Equal(t, channeltypes.OPEN, channel.State, "All returned channels should be OPEN")
			require.Equal(t, zctypes.PortID, channel.PortId, "All channels should use zoneconcierge port")
			openChannelIDs[channel.ChannelId] = true
		}

		// Verify correct channels are returned
		require.True(t, openChannelIDs["channel-open1"], "Should include channel-open1")
		require.True(t, openChannelIDs["channel-open2"], "Should include channel-open2")
	})

	// Test multiple open channels
	t.Run("multiple_open_channels", func(t *testing.T) {
		babylonApp, ctx, channelKeeper = setupChannelKeeperTest(t)

		expectedChannels := []struct {
			consumerID string
			channelID  string
		}{
			{"consumer-multi-1", "channel-multi-1"},
			{"consumer-multi-2", "channel-multi-2"},
			{"consumer-multi-3", "channel-multi-3"},
			{"consumer-multi-4", "channel-multi-4"},
			{"consumer-multi-5", "channel-multi-5"},
		}

		// Setup multiple open channels
		for _, expected := range expectedChannels {
			setupConsumerChannel(babylonApp, ctx, expected.consumerID, expected.channelID, channeltypes.OPEN)
		}

		openChannels := channelKeeper.GetAllOpenZCChannels(ctx)
		require.Len(t, openChannels, len(expectedChannels), "Should return all open channels")

		// Verify all channels are returned and have correct properties
		channelIDs := make(map[string]bool)
		for _, channel := range openChannels {
			require.Equal(t, channeltypes.OPEN, channel.State, "Channel %s should be OPEN", channel.ChannelId)
			require.Equal(t, zctypes.PortID, channel.PortId, "Channel %s should use zoneconcierge port", channel.ChannelId)
			channelIDs[channel.ChannelId] = true
		}

		// Verify all expected channels are present
		for _, expected := range expectedChannels {
			require.True(t, channelIDs[expected.channelID], "Should include %s", expected.channelID)
		}
	})

	// Test that non-zoneconcierge port channels are filtered out
	t.Run("filter_by_port_prefix", func(t *testing.T) {
		babylonApp, ctx, channelKeeper = setupChannelKeeperTest(t)

		// Setup zoneconcierge channel (should be included)
		consumerID1 := "consumer-zc"
		channelID1 := "channel-zc"
		setupConsumerChannel(babylonApp, ctx, consumerID1, channelID1, channeltypes.OPEN)

		// Setup channel with different port (should be filtered out)
		consumerID2 := "consumer-other"
		channelID2 := "channel-other"
		babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID2, &ibctmtypes.ClientState{})
		babylonApp.IBCKeeper.ConnectionKeeper.SetConnection(
			ctx, consumerID2, connectiontypes.ConnectionEnd{
				ClientId: consumerID2,
			},
		)
		// Set with different port ID (not zoneconcierge)
		babylonApp.IBCKeeper.ChannelKeeper.SetChannel(
			ctx, "different-port", channelID2, channeltypes.Channel{
				State:          channeltypes.OPEN,
				ConnectionHops: []string{consumerID2},
			},
		)

		openChannels := channelKeeper.GetAllOpenZCChannels(ctx)
		require.Len(t, openChannels, 1, "Should only return zoneconcierge port channels")
		require.Equal(t, channelID1, openChannels[0].ChannelId, "Should return the zoneconcierge channel")
		require.Equal(t, zctypes.PortID, openChannels[0].PortId, "Should use zoneconcierge port")
	})

	// Test return value consistency
	t.Run("return_value_consistency", func(t *testing.T) {
		babylonApp, ctx, channelKeeper = setupChannelKeeperTest(t)

		consumerID := "consumer-consistency"
		channelID := "channel-consistency"
		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.OPEN)

		// Call multiple times and verify consistency
		for i := 0; i < 5; i++ {
			openChannels := channelKeeper.GetAllOpenZCChannels(ctx)
			require.Len(t, openChannels, 1, "Should consistently return 1 channel on call %d", i)
			require.Equal(t, channelID, openChannels[0].ChannelId, "Channel ID should be consistent on call %d", i)
			require.Equal(t, channeltypes.OPEN, openChannels[0].State, "Channel state should be consistent on call %d", i)
		}
	})

	// Test slice behavior and immutability
	t.Run("slice_behavior", func(t *testing.T) {
		babylonApp, ctx, channelKeeper = setupChannelKeeperTest(t)

		consumerID := "consumer-slice"
		channelID := "channel-slice"
		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.OPEN)

		// Get first slice
		openChannels1 := channelKeeper.GetAllOpenZCChannels(ctx)
		require.Len(t, openChannels1, 1)

		// Get second slice
		openChannels2 := channelKeeper.GetAllOpenZCChannels(ctx)
		require.Len(t, openChannels2, 1)

		// Verify they contain the same data but are different slice instances
		require.Equal(t, openChannels1[0].ChannelId, openChannels2[0].ChannelId, "Channel IDs should match")
		require.Equal(t, openChannels1[0].State, openChannels2[0].State, "Channel states should match")

		// Modifying one slice should not affect the other (different instances)
		if len(openChannels1) > 0 {
			originalChannelID := openChannels1[0].ChannelId
			// This modification shouldn't affect the stored data or other calls
			openChannels1[0].ChannelId = "modified-channel"

			// Get fresh slice and verify original data is unchanged
			openChannels3 := channelKeeper.GetAllOpenZCChannels(ctx)
			require.Equal(t, originalChannelID, openChannels3[0].ChannelId, "Original data should be unchanged")
		}
	})
}

func TestGetAllOpenZCChannels_StateTransitions(t *testing.T) {
	// Test state transitions and their effect on GetAllOpenZCChannels
	t.Run("state_transitions", func(t *testing.T) {
		babylonApp, ctx, channelKeeper := setupChannelKeeperTest(t)
		consumerID := "consumer-transition"
		channelID := "channel-transition"

		// Initially no channels
		openChannels := channelKeeper.GetAllOpenZCChannels(ctx)
		require.Empty(t, openChannels, "Should start with no channels")

		// Create channel in INIT state
		setupConsumerChannel(babylonApp, ctx, consumerID, channelID, channeltypes.INIT)
		openChannels = channelKeeper.GetAllOpenZCChannels(ctx)
		require.Empty(t, openChannels, "Should not include INIT channels")

		// Transition to TRYOPEN
		babylonApp.IBCKeeper.ChannelKeeper.SetChannel(
			ctx, zctypes.PortID, channelID, channeltypes.Channel{
				State:          channeltypes.TRYOPEN,
				ConnectionHops: []string{consumerID},
			},
		)
		openChannels = channelKeeper.GetAllOpenZCChannels(ctx)
		require.Empty(t, openChannels, "Should not include TRYOPEN channels")

		// Transition to OPEN
		babylonApp.IBCKeeper.ChannelKeeper.SetChannel(
			ctx, zctypes.PortID, channelID, channeltypes.Channel{
				State:          channeltypes.OPEN,
				ConnectionHops: []string{consumerID},
			},
		)
		openChannels = channelKeeper.GetAllOpenZCChannels(ctx)
		require.Len(t, openChannels, 1, "Should include OPEN channels")
		require.Equal(t, channelID, openChannels[0].ChannelId, "Should return the opened channel")

		// Transition to CLOSED
		babylonApp.IBCKeeper.ChannelKeeper.SetChannel(
			ctx, zctypes.PortID, channelID, channeltypes.Channel{
				State:          channeltypes.CLOSED,
				ConnectionHops: []string{consumerID},
			},
		)
		openChannels = channelKeeper.GetAllOpenZCChannels(ctx)
		require.Empty(t, openChannels, "Should not include CLOSED channels")
	})
}
