package keeper_test

import (
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

// BenchmarkEndBlockerPerformance benchmarks the full EndBlocker execution
// with varying consumer counts to measure performance scalability
func BenchmarkEndBlockerPerformance(b *testing.B) {
	consumerCounts := []int{1, 10, 20, 30, 40, 50}

	for _, consumerCount := range consumerCounts {
		b.Run(fmt.Sprintf("consumers_%d", consumerCount), func(b *testing.B) {
			// Setup test environment
			babylonApp, ctx, zcKeeper := setupEndBlockerTest(b)

			// Setup comprehensive mock data for high coverage
			setupEndBlockerMockData(b, babylonApp, ctx, consumerCount)

			// Measure memory before benchmark
			var startMemStats, endMemStats runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&startMemStats)

			b.ResetTimer()
			b.ReportAllocs()

			startTime := time.Now()

			// Run EndBlocker 100 times as requested
			for i := 0; i < 100; i++ {
				_, err := zoneconcierge.EndBlocker(ctx, *zcKeeper)
				require.NoError(b, err)
			}

			duration := time.Since(startTime)

			b.StopTimer()
			runtime.GC()
			runtime.ReadMemStats(&endMemStats)

			// Calculate metrics
			avgDuration := duration / 100
			memoryUsed := endMemStats.TotalAlloc - startMemStats.TotalAlloc

			b.ReportMetric(float64(avgDuration.Nanoseconds()), "ns/op_avg")
			b.ReportMetric(float64(memoryUsed), "memory_bytes_used")
			b.ReportMetric(float64(consumerCount), "consumer_count")

			// Log detailed performance metrics
			b.Logf("Consumer Count: %d", consumerCount)
			b.Logf("Total Duration: %v", duration)
			b.Logf("Average Duration per EndBlocker call: %v", avgDuration)
			b.Logf("Memory Used: %d bytes", memoryUsed)
			b.Logf("Memory per consumer: %d bytes", memoryUsed/uint64(consumerCount))
		})
	}
}

// BenchmarkEndBlockerComponents benchmarks individual components of EndBlocker
// to identify performance bottlenecks
func BenchmarkEndBlockerComponents(b *testing.B) {
	consumerCounts := []int{10, 30, 50}

	for _, consumerCount := range consumerCounts {
		b.Run(fmt.Sprintf("GetConsumerChannelMap_%d_consumers", consumerCount), func(b *testing.B) {
			babylonApp, ctx, zcKeeper := setupEndBlockerTest(b)
			setupEndBlockerMockData(b, babylonApp, ctx, consumerCount)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := zcKeeper.GetConsumerChannelMap(ctx)
				require.NoError(b, err)
			}
		})

		b.Run(fmt.Sprintf("BroadcastBTCHeaders_%d_consumers", consumerCount), func(b *testing.B) {
			babylonApp, ctx, zcKeeper := setupEndBlockerTest(b)
			setupEndBlockerMockData(b, babylonApp, ctx, consumerCount)

			consumerChannelMap, err := zcKeeper.GetConsumerChannelMap(ctx)
			require.NoError(b, err)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := zcKeeper.BroadcastBTCHeaders(ctx, consumerChannelMap)
				require.NoError(b, err)
			}
		})

		b.Run(fmt.Sprintf("BroadcastBTCStakingEvents_%d_consumers", consumerCount), func(b *testing.B) {
			babylonApp, ctx, zcKeeper := setupEndBlockerTest(b)
			setupEndBlockerMockData(b, babylonApp, ctx, consumerCount)

			consumerChannelMap, err := zcKeeper.GetConsumerChannelMap(ctx)
			require.NoError(b, err)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := zcKeeper.BroadcastBTCStakingConsumerEvents(ctx, consumerChannelMap)
				require.NoError(b, err)
			}
		})
	}
}

// BenchmarkEndBlockerMemoryProfile provides detailed memory profiling
func BenchmarkEndBlockerMemoryProfile(b *testing.B) {
	consumerCounts := []int{1, 10, 20, 30, 40, 50}

	for _, consumerCount := range consumerCounts {
		b.Run(fmt.Sprintf("MemoryProfile_%d_consumers", consumerCount), func(b *testing.B) {
			babylonApp, ctx, zcKeeper := setupEndBlockerTest(b)
			setupEndBlockerMockData(b, babylonApp, ctx, consumerCount)

			var memStats runtime.MemStats
			measurements := make([]uint64, 100)

			for i := 0; i < 100; i++ {
				runtime.GC()
				runtime.ReadMemStats(&memStats)
				beforeAlloc := memStats.TotalAlloc

				_, err := zoneconcierge.EndBlocker(ctx, *zcKeeper)
				require.NoError(b, err)

				runtime.ReadMemStats(&memStats)
				measurements[i] = memStats.TotalAlloc - beforeAlloc
			}

			// Calculate statistics
			var total, min, max uint64
			min = measurements[0]
			max = measurements[0]

			for _, m := range measurements {
				total += m
				if m < min {
					min = m
				}
				if m > max {
					max = m
				}
			}

			avg := total / 100

			b.Logf("Consumer Count: %d", consumerCount)
			b.Logf("Memory per EndBlocker - Avg: %d bytes, Min: %d bytes, Max: %d bytes", avg, min, max)
			b.Logf("Memory per consumer - Avg: %d bytes", avg/uint64(consumerCount))
		})
	}
}

// Helper functions for EndBlocker benchmark setup

func setupEndBlockerTest(b *testing.B) (*app.BabylonApp, sdk.Context, *keeper.Keeper) {
	b.Helper()

	babylonApp := app.Setup(&testing.T{}, false)
	ctx := babylonApp.NewContext(false)
	zcKeeper := &babylonApp.ZoneConciergeKeeper

	return babylonApp, ctx, zcKeeper
}

func setupEndBlockerMockData(b *testing.B, babylonApp *app.BabylonApp, ctx sdk.Context, consumerCount int) {
	b.Helper()

	// Setup BTC chain with sufficient headers for comprehensive testing
	setupBTCChainForEndBlocker(b, babylonApp, ctx, 200)

	// Setup consumers with comprehensive mock data
	setupConsumersForEndBlocker(b, babylonApp, ctx, consumerCount)

	// Setup BTC staking events for each consumer
	setupBTCStakingEventsForEndBlocker(b, babylonApp, ctx, consumerCount)
}

func setupBTCChainForEndBlocker(b *testing.B, babylonApp *app.BabylonApp, ctx sdk.Context, chainLength uint32) {
	b.Helper()

	r := rand.New(rand.NewSource(54321)) // Fixed seed for reproducible benchmarks
	btcKeeper := &babylonApp.BTCLightClientKeeper

	// Initialize BTC genesis if not exists
	initHeader := btcKeeper.GetHeaderByHeight(ctx, 0)
	if initHeader == nil {
		genesisHeader, err := app.SignetBtcHeaderGenesis(babylonApp.AppCodec())
		require.NoError(b, err)

		if btcKeeper.GetBaseBTCHeader(ctx) == nil {
			genesisHeader.Height = 0
			btcKeeper.SetBaseBTCHeader(ctx, *genesisHeader)
		}

		initHeader = btcKeeper.GetHeaderByHeight(ctx, 0)
		require.NotNil(b, initHeader)
	}

	// Generate and insert BTC headers
	randomChain := datagen.NewBTCHeaderChainFromParentInfo(r, initHeader, chainLength)
	err := btcKeeper.InsertHeadersWithHookAndEvents(ctx, randomChain.ChainToBytes())
	require.NoError(b, err)

	// Verify chain tip
	tip := btcKeeper.GetTipInfo(ctx)
	chainTipInfo := randomChain.GetTipInfo()
	require.Equal(b, tip.Height, chainTipInfo.Height)
}

func setupConsumersForEndBlocker(b *testing.B, babylonApp *app.BabylonApp, ctx sdk.Context, consumerCount int) {
	b.Helper()

	for i := 0; i < consumerCount; i++ {
		consumerID := fmt.Sprintf("endblocker-consumer-%d", i)

		// Register consumer
		consumerRegister := &bsctypes.ConsumerRegister{
			ConsumerId:          consumerID,
			ConsumerName:        fmt.Sprintf("EndBlocker Benchmark Consumer %d", i),
			ConsumerDescription: fmt.Sprintf("Consumer for EndBlocker benchmark testing %d", i),
			ConsumerMetadata: &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
				CosmosConsumerMetadata: &bsctypes.CosmosConsumerMetadata{
					ChannelId: fmt.Sprintf("channel-%d", i),
				},
			},
		}
		err := babylonApp.BTCStkConsumerKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(b, err)

		// Setup IBC infrastructure
		setupIBCInfrastructureForEndBlocker(b, babylonApp, ctx, consumerID, i)

		// Setup varying last sent segments to trigger different logic paths
		if i%3 == 0 { //nolint:gocritic
			// Some consumers have no previous headers (first-time broadcast)
			continue
		} else if i%3 == 1 {
			// Some consumers have recent headers (incremental broadcast)
			tipHeight := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx).Height
			if tipHeight > 10 {
				startHeight := tipHeight - 10
				headers := make([]*btclctypes.BTCHeaderInfo, 0, 10)
				for h := startHeight; h < tipHeight; h++ {
					if header := babylonApp.BTCLightClientKeeper.GetHeaderByHeight(ctx, h); header != nil {
						headers = append(headers, header)
					}
				}
				if len(headers) > 0 {
					segment := &types.BTCChainSegment{
						BtcHeaders: headers,
					}
					babylonApp.ZoneConciergeKeeper.SetBSNLastSentSegment(ctx, consumerID, segment)
				}
			}
		} else {
			// Some consumers have older headers (larger broadcast needed)
			tipHeight := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx).Height
			if tipHeight > 50 {
				startHeight := tipHeight - 50
				headers := make([]*btclctypes.BTCHeaderInfo, 0, 5)
				for h := startHeight; h < startHeight+5; h++ {
					if header := babylonApp.BTCLightClientKeeper.GetHeaderByHeight(ctx, h); header != nil {
						headers = append(headers, header)
					}
				}
				if len(headers) > 0 {
					segment := &types.BTCChainSegment{
						BtcHeaders: headers,
					}
					babylonApp.ZoneConciergeKeeper.SetBSNLastSentSegment(ctx, consumerID, segment)
				}
			}
		}
	}
}

func setupIBCInfrastructureForEndBlocker(b *testing.B, babylonApp *app.BabylonApp, ctx sdk.Context, consumerID string, index int) {
	b.Helper()

	// Setup IBC client
	babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID, &ibctmtypes.ClientState{})

	// Setup connection
	connectionID := fmt.Sprintf("endblocker-connection-%d", index)
	babylonApp.IBCKeeper.ConnectionKeeper.SetConnection(
		ctx, connectionID, connectiontypes.ConnectionEnd{
			ClientId: consumerID,
		},
	)

	// Setup channel
	channelID := fmt.Sprintf("channel-%d", index)
	portID := types.PortID

	babylonApp.IBCKeeper.ChannelKeeper.SetChannel(
		ctx, portID, channelID, channeltypes.Channel{
			State:          channeltypes.OPEN,
			ConnectionHops: []string{connectionID},
		},
	)
}

func setupBTCStakingEventsForEndBlocker(b *testing.B, babylonApp *app.BabylonApp, ctx sdk.Context, consumerCount int) {
	b.Helper()

	// Create mock BTC staking events for some consumers to trigger broadcast logic
	for i := 0; i < consumerCount; i++ {
		if i%2 == 0 { // Every other consumer has staking events
			consumerID := fmt.Sprintf("endblocker-consumer-%d", i)

			// Create mock BTC staking IBC packet
			// Create mock events to trigger various logic paths

			newFpEvent := &bstypes.BTCStakingConsumerEvent{
				Event: &bstypes.BTCStakingConsumerEvent_NewFp{
					NewFp: &bstypes.NewFinalityProvider{
						BtcPkHex:   fmt.Sprintf("%x", datagen.GenRandomByteArray(r, 33)),
						Commission: "0.1",
						Addr:       fmt.Sprintf("addr-%d", i),
						BsnId:      consumerID,
					},
				},
			}

			activeDel := &bstypes.BTCStakingConsumerEvent{
				Event: &bstypes.BTCStakingConsumerEvent_ActiveDel{
					ActiveDel: &bstypes.ActiveBTCDelegation{
						BtcPkHex:         fmt.Sprintf("%x", datagen.GenRandomByteArray(r, 33)),
						FpBtcPkList:      []string{fmt.Sprintf("%x", datagen.GenRandomByteArray(r, 33))},
						StartHeight:      uint32(i * 10),
						EndHeight:        uint32(i*10 + 100),
						TotalSat:         uint64(1000000 + i*10000),
						StakingTx:        datagen.GenRandomByteArray(r, 200),
						SlashingTx:       datagen.GenRandomByteArray(r, 150),
						StakingOutputIdx: 0,
						UnbondingTime:    144,
						ParamsVersion:    1,
					},
				},
			}

			events := []*bstypes.BTCStakingConsumerEvent{newFpEvent, activeDel}
			err := babylonApp.BTCStakingKeeper.AddBTCStakingConsumerEvents(ctx, consumerID, events)
			require.NoError(b, err)
		}
	}
}

var r = rand.New(rand.NewSource(12345))
