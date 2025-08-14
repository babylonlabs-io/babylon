package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

func FuzzConsumerRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// generate a random consumer register
		consumerRegister := datagen.GenRandomCosmosConsumerRegister(r)

		// check that the consumer is not registered
		isRegistered := bscKeeper.IsConsumerRegistered(ctx, consumerRegister.ConsumerId)
		require.False(t, isRegistered)

		// Check that the consumer is not registered
		consumerRegister2, err := bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.Error(t, err)
		require.Nil(t, consumerRegister2)

		// Register the consumer
		err = bscKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(t, err)
		// check that the consumer is registered
		consumerRegister2, err = bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, consumerRegister.ConsumerId, consumerRegister2.ConsumerId)
		require.Equal(t, consumerRegister.ConsumerName, consumerRegister2.ConsumerName)
		require.Equal(t, consumerRegister.ConsumerDescription, consumerRegister2.ConsumerDescription)
	})
}

func TestCosmosConsumerMetadataValidation(t *testing.T) {
	babylonApp := app.Setup(t, false)
	bscKeeper := babylonApp.BTCStkConsumerKeeper
	ctx := babylonApp.NewContext(false)

	// Test NewCosmosConsumerRegister creates consumer with non-nil metadata
	t.Run("NewCosmosConsumerRegister_creates_non_nil_metadata", func(t *testing.T) {
		consumerRegister := types.NewCosmosConsumerRegister(
			"test-consumer-1",
			"Test Consumer",
			"Test Description",
			math.LegacyNewDecWithPrec(5, 2), // 0.05
		)

		// Verify metadata is not nil
		require.NotNil(t, consumerRegister.GetCosmosConsumerMetadata())

		// Verify the consumer is considered a cosmos consumer
		require.True(t, consumerRegister.GetCosmosConsumerMetadata() != nil)
	})

	// Test empty case - no consumers registered
	t.Run("empty_registry", func(t *testing.T) {
		cosmosConsumers := bscKeeper.GetAllRegisteredCosmosConsumers(ctx)
		require.Empty(t, cosmosConsumers, "Should return empty slice when no consumers are registered")
		require.Len(t, cosmosConsumers, 0, "Should return a slice of length 0 when no consumers are registered")
	})

	// Test consumer without channel_id is still included in GetAllRegisteredCosmosConsumers
	t.Run("consumer_without_channel_id_included_in_GetAllRegisteredCosmosConsumers", func(t *testing.T) {
		// Create consumer with empty channel_id
		consumerRegister := types.NewCosmosConsumerRegister(
			"test-consumer-2",
			"Test Consumer 2",
			"Test Description 2",
			math.LegacyNewDecWithPrec(10, 2), // 0.10
		)

		// Register the consumer
		err := bscKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(t, err)

		// Get all registered cosmos consumers
		cosmosConsumers := bscKeeper.GetAllRegisteredCosmosConsumers(ctx)

		// Verify the consumer is included even without channel_id
		found := false
		for _, consumer := range cosmosConsumers {
			if consumer.ConsumerId == "test-consumer-2" {
				found = true
				break
			}
		}
		require.False(t, found, "Consumer should not be found in GetAllRegisteredCosmosConsumers when without channel_id")
	})

	// Test consumer with channel_id is included in GetAllRegisteredCosmosConsumers
	t.Run("consumer_with_channel_id_included_in_GetAllRegisteredCosmosConsumers", func(t *testing.T) {
		// Create consumer with channel_id set
		consumerRegister := types.NewCosmosConsumerRegister(
			"test-consumer-3",
			"Test Consumer 3",
			"Test Description 3",
			math.LegacyNewDecWithPrec(15, 2), // 0.15
		)
		consumerRegister.GetCosmosConsumerMetadata().ChannelId = "channel-123"

		// Register the consumer
		err := bscKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(t, err)

		// Get all registered cosmos consumers
		cosmosConsumers := bscKeeper.GetAllRegisteredCosmosConsumers(ctx)

		// Verify the consumer is included with channel_id
		found := false
		for _, consumer := range cosmosConsumers {
			if consumer.ConsumerId == "test-consumer-3" {
				found = true
				// Verify metadata exists and channel_id is set
				require.NotNil(t, consumer.GetCosmosConsumerMetadata())
				require.Equal(t, "channel-123", consumer.GetCosmosConsumerMetadata().ChannelId)
				break
			}
		}
		require.True(t, found, "Consumer should be found in GetAllRegisteredCosmosConsumers with channel_id")
	})

	// Test rollup consumer is NOT included in GetAllRegisteredCosmosConsumers
	t.Run("rollup_consumer_not_included_in_GetAllRegisteredCosmosConsumers", func(t *testing.T) {
		// Create rollup consumer
		rollupRegister := types.NewRollupConsumerRegister(
			"test-rollup-1",
			"Test Rollup",
			"Test Rollup Description",
			"0x1234567890abcdef",
			math.LegacyNewDecWithPrec(20, 2), // 0.20
		)

		// Register the rollup consumer
		err := bscKeeper.RegisterConsumer(ctx, rollupRegister)
		require.NoError(t, err)

		// Get all registered cosmos consumers
		cosmosConsumers := bscKeeper.GetAllRegisteredCosmosConsumers(ctx)

		// Verify the rollup consumer is NOT included
		found := false
		for _, consumer := range cosmosConsumers {
			if consumer.ConsumerId == "test-rollup-1" {
				found = true
				break
			}
		}
		require.False(t, found, "Rollup consumer should NOT be found in GetAllRegisteredCosmosConsumers")

		// Verify rollup consumer has nil cosmos metadata
		require.Nil(t, rollupRegister.GetCosmosConsumerMetadata())
	})

	// Test multiple mixed consumer types
	t.Run("multiple_mixed_consumers", func(t *testing.T) {
		babylonApp = app.Setup(t, false)
		bscKeeper = babylonApp.BTCStkConsumerKeeper
		ctx = babylonApp.NewContext(false)

		// Create multiple cosmos consumers with channel IDs
		cosmos1 := types.NewCosmosConsumerRegister(
			"cosmos-1", "Cosmos 1", "Description 1", math.LegacyNewDecWithPrec(5, 2))
		cosmos1.GetCosmosConsumerMetadata().ChannelId = "channel-cosmos-1"

		cosmos2 := types.NewCosmosConsumerRegister(
			"cosmos-2", "Cosmos 2", "Description 2", math.LegacyNewDecWithPrec(10, 2))
		cosmos2.GetCosmosConsumerMetadata().ChannelId = "channel-cosmos-2"

		// Create cosmos consumer without channel ID
		cosmos3 := types.NewCosmosConsumerRegister(
			"cosmos-3", "Cosmos 3", "Description 3", math.LegacyNewDecWithPrec(15, 2))
		// cosmos3 has empty channel ID

		// Create rollup consumer
		rollup1 := types.NewRollupConsumerRegister(
			"rollup-1", "Rollup 1", "Description 4", "0xabcdef", math.LegacyNewDecWithPrec(20, 2))

		// Register all consumers
		err := bscKeeper.RegisterConsumer(ctx, cosmos1)
		require.NoError(t, err)
		err = bscKeeper.RegisterConsumer(ctx, cosmos2)
		require.NoError(t, err)
		err = bscKeeper.RegisterConsumer(ctx, cosmos3)
		require.NoError(t, err)
		err = bscKeeper.RegisterConsumer(ctx, rollup1)
		require.NoError(t, err)

		cosmosConsumers := bscKeeper.GetAllRegisteredCosmosConsumers(ctx)
		require.Len(t, cosmosConsumers, 2, "Should return only cosmos consumers with channel IDs")

		// Check that returned consumers are correct
		consumerIDs := make(map[string]bool)
		for _, consumer := range cosmosConsumers {
			consumerIDs[consumer.ConsumerId] = true
			require.NotNil(t, consumer.GetCosmosConsumerMetadata(), "All returned consumers should have cosmos metadata")
			require.NotEmpty(t, consumer.GetCosmosConsumerMetadata().ChannelId, "All returned consumers should have non-empty channel ID")
		}

		require.True(t, consumerIDs["cosmos-1"], "Should include cosmos-1")
		require.True(t, consumerIDs["cosmos-2"], "Should include cosmos-2")
		require.False(t, consumerIDs["cosmos-3"], "Should exclude cosmos-3 (no channel ID)")
		require.False(t, consumerIDs["rollup-1"], "Should exclude rollup-1")
	})
}

func TestCosmosConsumerIdentificationStrategies(t *testing.T) {
	// This test demonstrates different strategies for identifying cosmos consumers
	// and whether metadata != nil check is sufficient vs requiring channel_id validation

	t.Run("metadata_nil_check_vs_channel_id_validation", func(t *testing.T) {
		// Strategy 1: Check metadata != nil (current approach)
		cosmosConsumerWithoutChannel := types.NewCosmosConsumerRegister(
			"cosmos-no-channel", "Cosmos No Channel", "Description", math.LegacyNewDecWithPrec(5, 2))
		cosmosConsumerWithChannel := types.NewCosmosConsumerRegister(
			"cosmos-with-channel", "Cosmos With Channel", "Description", math.LegacyNewDecWithPrec(10, 2))
		cosmosConsumerWithChannel.GetCosmosConsumerMetadata().ChannelId = "channel-456"

		rollupConsumer := types.NewRollupConsumerRegister(
			"rollup-consumer", "Rollup Consumer", "Description", "0xabcdef", math.LegacyNewDecWithPrec(15, 2))

		// Test: metadata != nil
		t.Run("current_strategy_metadata_not_nil", func(t *testing.T) {
			// Both cosmos consumers should be identified as cosmos consumers
			require.True(t, cosmosConsumerWithoutChannel.GetCosmosConsumerMetadata() != nil,
				"Cosmos consumer without channel should be identified by metadata != nil")
			require.True(t, cosmosConsumerWithChannel.GetCosmosConsumerMetadata() != nil,
				"Cosmos consumer with channel should be identified by metadata != nil")

			// Rollup consumer should NOT be identified as cosmos consumer
			require.False(t, rollupConsumer.GetCosmosConsumerMetadata() != nil,
				"Rollup consumer should NOT be identified as cosmos consumer")
		})

		// Test: require channel_id
		t.Run("alternative_strategy_require_channel_id", func(t *testing.T) {
			// Only cosmos consumer with channel_id would be identified
			hasChannelId := func(cr *types.ConsumerRegister) bool {
				metadata := cr.GetCosmosConsumerMetadata()
				return metadata != nil && metadata.ChannelId != ""
			}

			require.False(t, hasChannelId(cosmosConsumerWithoutChannel),
				"Cosmos consumer without channel would be excluded with channel_id requirement")
			require.True(t, hasChannelId(cosmosConsumerWithChannel),
				"Cosmos consumer with channel would be included with channel_id requirement")
			require.False(t, hasChannelId(rollupConsumer),
				"Rollup consumer would be excluded with channel_id requirement")
		})
	})
}
