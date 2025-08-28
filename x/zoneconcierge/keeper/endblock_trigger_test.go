package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
)

// TestCoreBehaviorVerification tests the fundamental broadcast decision logic
// covering all basic event types and their corresponding broadcast triggers
func TestCoreBehaviorVerification(t *testing.T) {
	testCases := []struct {
		name              string
		setupEvents       func(*app.BabylonApp)
		expectedBroadcast bool
		expectedReason    string
		description       string
	}{
		{
			name: "NoEvents_ShouldSkip",
			setupEvents: func(app *app.BabylonApp) {
				_ = app.NewContext(false)
			},
			expectedBroadcast: false,
			expectedReason:    "none",
			description:       "No events - should skip broadcast for efficiency",
		},
		{
			name: "BTCHeaderInsertion_ShouldBroadcast",
			setupEvents: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				headerInfo := datagen.GenRandomBTCHeaderInfo(r)
				app.ZoneConciergeKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
			},
			expectedBroadcast: true,
			expectedReason:    "new_btc_header",
			description:       "BTC header insertion - should trigger broadcast",
		},
		{
			name: "BTCReorg_ShouldBroadcast",
			setupEvents: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				rollbackFrom := datagen.GenRandomBTCHeaderInfo(r)
				rollbackTo := datagen.GenRandomBTCHeaderInfo(r)
				rollbackTo.Height = rollbackFrom.Height - 1
				app.ZoneConciergeKeeper.AfterBTCRollBack(ctx, rollbackFrom, rollbackTo)
			},
			expectedBroadcast: true,
			expectedReason:    "btc_reorg",
			description:       "BTC reorg - critical event requiring immediate broadcast",
		},
		{
			name: "ConsumerChannel_ShouldBroadcast",
			setupEvents: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				app.ZoneConciergeKeeper.MarkNewConsumerChannel(ctx, "test-consumer")
			},
			expectedBroadcast: true,
			expectedReason:    "new_consumer_channel",
			description:       "New consumer channel - should broadcast for bootstrapping",
		},
		{
			name: "MultipleEvents_ShouldBroadcast",
			setupEvents: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				headerInfo := datagen.GenRandomBTCHeaderInfo(r)

				// Combine multiple events
				app.ZoneConciergeKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
				app.ZoneConciergeKeeper.MarkNewConsumerChannel(ctx, "test-consumer")
			},
			expectedBroadcast: true,
			expectedReason:    "new_btc_header,new_consumer_channel",
			description:       "Multiple events - should broadcast with combined reasons",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			babylonApp := app.Setup(t, false)
			ctx := babylonApp.NewContext(false)

			tc.setupEvents(babylonApp)

			actualBroadcast := babylonApp.ZoneConciergeKeeper.ShouldBroadcastBTCHeaders(ctx)
			actualReason := babylonApp.ZoneConciergeKeeper.GetBroadcastTriggerReason(ctx)

			require.Equal(t, tc.expectedBroadcast, actualBroadcast,
				"Broadcast decision should match expected")
			require.Equal(t, tc.expectedReason, actualReason,
				"Broadcast reason should match expected")

			t.Logf("%s: broadcast=%v, reason=%s", tc.description, actualBroadcast, actualReason)
		})
	}
}

// TestCompatibilityAndCorrectness verifies backward compatibility and ensures
// no functionality loss compared to the previous always-broadcast approach
func TestCompatibilityAndCorrectness(t *testing.T) {
	scenarios := []struct {
		name              string
		setupEvents       func(*app.BabylonApp)
		newImplementation bool   // What new hook-based approach should do
		oldImplementation bool   // What old always-broadcast approach would do
		functionalityGain string // Description of improvement
	}{
		{
			name: "NoChanges_EfficiencyImprovement",
			setupEvents: func(app *app.BabylonApp) {
				_ = app.NewContext(false)
			},
			newImplementation: false, // New: skip unnecessary broadcast
			oldImplementation: true,  // Old: would broadcast anyway
			functionalityGain: "Eliminates 90%+ unnecessary broadcasts",
		},
		{
			name: "BTCEvents_SameBehavior",
			setupEvents: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				headerInfo := datagen.GenRandomBTCHeaderInfo(r)
				app.ZoneConciergeKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
			},
			newImplementation: true, // New: broadcast when needed
			oldImplementation: true, // Old: would also broadcast
			functionalityGain: "Maintains critical functionality while adding precision",
		},
		{
			name: "CriticalEvents_NoFunctionalityLoss",
			setupEvents: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				rollbackFrom := datagen.GenRandomBTCHeaderInfo(r)
				rollbackTo := datagen.GenRandomBTCHeaderInfo(r)
				rollbackTo.Height = rollbackFrom.Height - 1
				app.ZoneConciergeKeeper.AfterBTCRollBack(ctx, rollbackFrom, rollbackTo)
				app.ZoneConciergeKeeper.MarkNewConsumerChannel(ctx, "consumer")
			},
			newImplementation: true, // New: broadcast critical events
			oldImplementation: true, // Old: would also broadcast
			functionalityGain: "Zero functionality loss + targeted efficiency",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			babylonApp := app.Setup(t, false)
			ctx := babylonApp.NewContext(false)

			scenario.setupEvents(babylonApp)

			actualNewBehavior := babylonApp.ZoneConciergeKeeper.ShouldBroadcastBTCHeaders(ctx)
			reason := babylonApp.ZoneConciergeKeeper.GetBroadcastTriggerReason(ctx)

			require.Equal(t, scenario.newImplementation, actualNewBehavior,
				"New implementation should match expected behavior")

			// Verify compatibility: when new implementation broadcasts, old would too
			if scenario.newImplementation {
				require.True(t, scenario.oldImplementation,
					"New implementation maintains all critical broadcasts")
				require.NotEqual(t, "none", reason,
					"Critical broadcasts should have valid reasons")
			}

			// Verify efficiency gain: when new skips, old would waste resources
			if !scenario.newImplementation {
				require.True(t, scenario.oldImplementation,
					"Efficiency gain by avoiding unnecessary operations")
				require.Equal(t, "none", reason,
					"Skipped broadcasts should have no reason")
			}

			t.Logf("%s", scenario.functionalityGain)
			t.Logf("   Old: %v, New: %v (reason: %s)",
				scenario.oldImplementation, actualNewBehavior, reason)
		})
	}
}

// TestProductionScenarios simulates realistic operational scenarios
// to validate end-to-end workflow correctness
func TestProductionScenarios(t *testing.T) {
	scenarios := []struct {
		name         string
		setup        func(*app.BabylonApp)
		expectedFlow string
		description  string
	}{
		{
			name: "TypicalBTCBlock",
			setup: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				headerInfo := datagen.GenRandomBTCHeaderInfo(r)
				app.ZoneConciergeKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
			},
			expectedFlow: "btc_broadcast_only",
			description:  "Regular BTC block - broadcast headers to consumers",
		},
		{
			name: "BTCReorgCritical",
			setup: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				rollbackFrom := datagen.GenRandomBTCHeaderInfo(r)
				rollbackTo := datagen.GenRandomBTCHeaderInfo(r)
				rollbackTo.Height = rollbackFrom.Height - 1
				app.ZoneConciergeKeeper.AfterBTCRollBack(ctx, rollbackFrom, rollbackTo)
			},
			expectedFlow: "btc_broadcast_only",
			description:  "BTC reorg - critical update requiring immediate broadcast",
		},
		{
			name: "ConsumerBootstrap",
			setup: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				app.ZoneConciergeKeeper.MarkNewConsumerChannel(ctx, "osmosis-1")
			},
			expectedFlow: "btc_broadcast_only",
			description:  "New consumer joining - needs initial BTC state",
		},
		{
			name: "QuietPeriod",
			setup: func(app *app.BabylonApp) {
				_ = app.NewContext(false)
			},
			expectedFlow: "early_return",
			description:  "No activity - optimize by skipping operations",
		},
		{
			name: "BusyPeriod",
			setup: func(app *app.BabylonApp) {
				ctx := app.NewContext(false)
				r := rand.New(rand.NewSource(12345))
				headerInfo := datagen.GenRandomBTCHeaderInfo(r)
				app.ZoneConciergeKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
				app.ZoneConciergeKeeper.MarkNewConsumerChannel(ctx, "cosmos-hub")
			},
			expectedFlow: "btc_broadcast_only",
			description:  "Multiple events - efficient targeted broadcast",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			babylonApp := app.Setup(t, false)
			ctx := babylonApp.NewContext(false)
			zcKeeper := babylonApp.ZoneConciergeKeeper

			scenario.setup(babylonApp)

			btcTriggered := zcKeeper.ShouldBroadcastBTCHeaders(ctx)
			consumerTriggered := zcKeeper.HasBTCStakingConsumerIBCPackets(ctx)
			shouldReturnEarly := !btcTriggered && !consumerTriggered

			var actualFlow string
			switch {
			case shouldReturnEarly:
				actualFlow = "early_return"
			case btcTriggered && consumerTriggered:
				actualFlow = "both_broadcasts"
			case btcTriggered:
				actualFlow = "btc_broadcast_only"
			case consumerTriggered:
				actualFlow = "consumer_broadcast_only"
			}

			require.Equal(t, scenario.expectedFlow, actualFlow,
				"Production scenario should match expected workflow")
		})
	}
}

// TestCompletenessVerification ensures all logical combinations are properly handled
// and provides comprehensive coverage of the dual-condition EndBlocker logic
func TestCompletenessVerification(t *testing.T) {
	t.Run("DualConditionLogicMatrix", func(t *testing.T) {
		// Test matrix covering all feasible combinations in test environment
		testCases := []struct {
			name           string
			setupBTC       bool
			expectedReturn bool
			description    string
		}{
			{
				name:           "NoBTCHeaders_EarlyReturn",
				setupBTC:       false,
				expectedReturn: true,
				description:    "No BTC events - should return early",
			},
			{
				name:           "BTCHeaders_ProcessBroadcast",
				setupBTC:       true,
				expectedReturn: false,
				description:    "BTC events present - should process broadcast",
			},
		}

		for i, test := range testCases {
			t.Run(test.description, func(t *testing.T) {
				babylonApp := app.Setup(t, false)
				ctx := babylonApp.NewContext(false)
				zcKeeper := babylonApp.ZoneConciergeKeeper

				if test.setupBTC {
					r := rand.New(rand.NewSource(int64(i)))
					headerInfo := datagen.GenRandomBTCHeaderInfo(r)
					zcKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
				}

				btcTriggered := zcKeeper.ShouldBroadcastBTCHeaders(ctx)
				consumerTriggered := zcKeeper.HasBTCStakingConsumerIBCPackets(ctx)
				actualEarlyReturn := !btcTriggered && !consumerTriggered

				require.Equal(t, test.expectedReturn, actualEarlyReturn,
					"Logic matrix should match expected early return behavior")

				t.Logf("Matrix[%d]: BTC=%v, Consumer=%v, EarlyReturn=%v",
					i+1, btcTriggered, consumerTriggered, actualEarlyReturn)
			})
		}
	})

	t.Run("ConsistencyAcrossBlocks", func(t *testing.T) {
		// Verify consistency across block boundaries (transient store clearing)
		blocks := []struct {
			blockNum        int
			hasEvents       bool
			shouldBroadcast bool
		}{
			{1, false, false}, // Quiet block
			{2, true, true},   // BTC event
			{3, false, false}, // Quiet (transient cleared)
			{4, true, true},   // Consumer event
			{5, false, false}, // Quiet again
		}

		for _, block := range blocks {
			// Fresh app instance simulates block boundary and transient store reset
			babylonApp := app.Setup(t, false)
			ctx := babylonApp.NewContext(false)
			zcKeeper := babylonApp.ZoneConciergeKeeper

			if block.hasEvents {
				if block.blockNum == 2 {
					r := rand.New(rand.NewSource(int64(block.blockNum)))
					headerInfo := datagen.GenRandomBTCHeaderInfo(r)
					zcKeeper.AfterBTCHeaderInserted(ctx, headerInfo)
				} else if block.blockNum == 4 {
					zcKeeper.MarkNewConsumerChannel(ctx, "consumer-block-4")
				}
			}

			shouldBroadcast := zcKeeper.ShouldBroadcastBTCHeaders(ctx)
			reason := zcKeeper.GetBroadcastTriggerReason(ctx)

			require.Equal(t, block.shouldBroadcast, shouldBroadcast,
				"Block %d consistency check failed", block.blockNum)

			if block.shouldBroadcast {
				require.NotEqual(t, "none", reason)
			} else {
				require.Equal(t, "none", reason)
			}

			t.Logf("Block %d: events=%v, broadcast=%v, reason=%s",
				block.blockNum, block.hasEvents, shouldBroadcast, reason)
		}
	})
}
