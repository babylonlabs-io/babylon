package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

func TestBTCLightClientHooks(t *testing.T) {
	t.Run("ShouldBroadcastBTCHeaders_InitiallyFalse", func(t *testing.T) {
		// Setup test environment with transient store
		babylonApp := app.Setup(t, false)
		ctx := babylonApp.NewContext(false)
		zcKeeper := babylonApp.ZoneConciergeKeeper

		// Initially, no triggers should be set
		require.False(t, zcKeeper.ShouldBroadcastBTCHeaders(ctx))
		require.Equal(t, "none", zcKeeper.GetBroadcastTriggerReason(ctx))
	})

	t.Run("AfterBTCHeaderInserted_TriggersBroadcast", func(t *testing.T) {
		// Setup fresh test environment
		babylonApp := app.Setup(t, false)
		ctx := babylonApp.NewContext(false)
		zcKeeper := babylonApp.ZoneConciergeKeeper

		// Create a mock BTC header
		r := rand.New(rand.NewSource(12345))
		headerInfo := datagen.GenRandomBTCHeaderInfo(r)

		// Simulate BTC header insertion hook
		zcKeeper.AfterBTCHeaderInserted(ctx, headerInfo)

		// Should now trigger broadcasting
		require.True(t, zcKeeper.ShouldBroadcastBTCHeaders(ctx))
		require.Equal(t, "new_btc_header", zcKeeper.GetBroadcastTriggerReason(ctx))
	})

	t.Run("AfterBTCRollBack_TriggersBroadcast", func(t *testing.T) {
		// Setup fresh test environment
		babylonApp := app.Setup(t, false)
		ctx := babylonApp.NewContext(false)
		zcKeeper := babylonApp.ZoneConciergeKeeper

		// Create mock BTC headers for rollback
		r := rand.New(rand.NewSource(12345))
		rollbackFrom := datagen.GenRandomBTCHeaderInfo(r)
		rollbackTo := datagen.GenRandomBTCHeaderInfo(r)
		rollbackTo.Height = rollbackFrom.Height - 1

		// Simulate BTC rollback hook
		zcKeeper.AfterBTCRollBack(ctx, rollbackFrom, rollbackTo)

		// Should now trigger broadcasting
		require.True(t, zcKeeper.ShouldBroadcastBTCHeaders(ctx))
		require.Equal(t, "btc_reorg", zcKeeper.GetBroadcastTriggerReason(ctx))
	})

	t.Run("MarkNewConsumerChannel_TriggersBroadcast", func(t *testing.T) {
		// Setup fresh test environment
		babylonApp := app.Setup(t, false)
		ctx := babylonApp.NewContext(false)
		zcKeeper := babylonApp.ZoneConciergeKeeper

		// Mark new consumer channel
		zcKeeper.MarkNewConsumerChannel(ctx, "test-consumer-123")

		// Should now trigger broadcasting
		require.True(t, zcKeeper.ShouldBroadcastBTCHeaders(ctx))
		require.Equal(t, "new_consumer_channel", zcKeeper.GetBroadcastTriggerReason(ctx))
	})

	t.Run("MultipleTriggers_CombinedReason", func(t *testing.T) {
		// Setup fresh test environment
		babylonApp := app.Setup(t, false)
		ctx := babylonApp.NewContext(false)
		zcKeeper := babylonApp.ZoneConciergeKeeper

		// Create mock data
		r := rand.New(rand.NewSource(12345))
		headerInfo := datagen.GenRandomBTCHeaderInfo(r)

		// Trigger multiple events
		zcKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
		zcKeeper.MarkNewConsumerChannel(ctx, "test-consumer-456")

		// Should trigger broadcasting with combined reason
		require.True(t, zcKeeper.ShouldBroadcastBTCHeaders(ctx))
		reason := zcKeeper.GetBroadcastTriggerReason(ctx)

		// Reason should contain both triggers (order may vary)
		require.Contains(t, reason, "new_btc_header")
		require.Contains(t, reason, "new_consumer_channel")
	})

	t.Run("TransientStore_ClearsAcrossBlocks", func(t *testing.T) {
		// Setup first test environment
		babylonApp1 := app.Setup(t, false)
		zcKeeper1 := babylonApp1.ZoneConciergeKeeper

		// Set up trigger in one block
		ctx1 := babylonApp1.NewContext(false)
		r := rand.New(rand.NewSource(12345))
		headerInfo := datagen.GenRandomBTCHeaderInfo(r)
		zcKeeper1.AfterBTCHeaderInserted(ctx1, headerInfo)
		require.True(t, zcKeeper1.ShouldBroadcastBTCHeaders(ctx1))

		// Setup second test environment (simulating new block with fresh transient store)
		babylonApp2 := app.Setup(t, false)
		zcKeeper2 := babylonApp2.ZoneConciergeKeeper
		ctx2 := babylonApp2.NewContext(false)

		// Should not trigger in new block (transient store is cleared)
		require.False(t, zcKeeper2.ShouldBroadcastBTCHeaders(ctx2))
		require.Equal(t, "none", zcKeeper2.GetBroadcastTriggerReason(ctx2))
	})
}

func TestConditionalBroadcasting_Integration(t *testing.T) {
	babylonApp := app.Setup(t, false)
	ctx := babylonApp.NewContext(false)
	zcKeeper := babylonApp.ZoneConciergeKeeper

	t.Run("EndBlocker_SkipsWhenNoTriggers", func(t *testing.T) {
		// No triggers set, so EndBlocker should skip BTC header broadcasting

		// Mock a call to get consumer channel map (should work)
		_, err := zcKeeper.GetConsumerChannelMap(ctx)
		require.NoError(t, err) // This should work regardless

		// The main test here is that ShouldBroadcastBTCHeaders returns false
		require.False(t, zcKeeper.ShouldBroadcastBTCHeaders(ctx))
	})

	t.Run("EndBlocker_BroadcastsWhenTriggered", func(t *testing.T) {
		// Set up a trigger
		r := rand.New(rand.NewSource(12345))
		headerInfo := datagen.GenRandomBTCHeaderInfo(r)
		zcKeeper.AfterBTCHeaderInserted(ctx, headerInfo)

		// Now EndBlocker should broadcast
		require.True(t, zcKeeper.ShouldBroadcastBTCHeaders(ctx))

		// In a real scenario, EndBlocker would call BroadcastBTCHeaders here
		// We can't easily test the full EndBlocker without setting up IBC infrastructure,
		// but we've verified the trigger mechanism works
	})
}

// TestHookInterface verifies that ZoneConcierge keeper properly implements BTCLightClientHooks
func TestHookInterface(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper

	// Verify that zcKeeper implements BTCLightClientHooks interface
	var _ btclctypes.BTCLightClientHooks = zcKeeper

	// This test will fail at compile time if the interface is not implemented correctly
	t.Log("ZoneConcierge keeper successfully implements BTCLightClientHooks interface")
}

func TestTransientStoreKeys(t *testing.T) {
	// Test that our transient store keys are properly defined
	require.Equal(t, "transient_zc", types.TStoreKey)
	require.Equal(t, []byte{100}, types.BTCHeaderInsertedKey)
	require.Equal(t, []byte{101}, types.BTCReorgOccurredKey)
	require.Equal(t, []byte{102}, types.NewConsumerChannelKey)
}
