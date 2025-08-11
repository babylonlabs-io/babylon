package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btclckeeper "github.com/babylonlabs-io/babylon/v3/x/btclightclient/keeper"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"
)

// BenchmarkBroadcastBTCHeaders benchmarks the BroadcastBTCHeaders function
// with different scales of consumers, channels, and BTC headers
func BenchmarkBroadcastBTCHeaders(b *testing.B) {
	benchmarkCases := []struct {
		name                string
		consumers           int
		channelsPerConsumer int
		btcChainLength      uint32
		headerCacheHits     bool // Whether to simulate cache hits by reusing headers
	}{
		{"Small_10c_1ch_50h_NoCacheHits", 10, 1, 50, false},
		{"Small_10c_2ch_50h_WithCacheHits", 10, 2, 50, true},
		{"Medium_50c_2ch_100h_NoCacheHits", 50, 2, 100, false},
		{"Medium_50c_3ch_100h_WithCacheHits", 50, 3, 100, true},
		{"Large_100c_2ch_200h_NoCacheHits", 100, 2, 200, false},
		{"Large_100c_4ch_200h_WithCacheHits", 100, 4, 200, true},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			app, ctx, zcKeeper := setupBTCHeadersTest(b)

			// Setup BTC chain with headers
			btcKeeper := &app.BTCLightClientKeeper
			setupBTCChain(b, app, ctx, btcKeeper, bc.btcChainLength)

			// Setup consumers and channels
			consumerChannels := setupBTCHeadersConsumersAndChannels(b, app, ctx, bc.consumers, bc.channelsPerConsumer, bc.headerCacheHits)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				err := zcKeeper.BroadcastBTCHeaders(ctx, consumerChannels)
				require.NoError(b, err)
			}
		})
	}
}

// Helper functions for BTC headers benchmark setup

func setupBTCHeadersTest(b *testing.B) (*app.BabylonApp, sdk.Context, *keeper.Keeper) {
	b.Helper()

	babylonApp := app.Setup(&testing.T{}, false)
	ctx := babylonApp.NewContext(false)
	zcKeeper := &babylonApp.ZoneConciergeKeeper

	return babylonApp, ctx, zcKeeper
}

func setupBTCChain(b *testing.B, babylonApp *app.BabylonApp, ctx sdk.Context, btcKeeper *btclckeeper.Keeper, chainLength uint32) *datagen.BTCHeaderPartialChain {
	b.Helper()

	r := rand.New(rand.NewSource(12345)) // Fixed seed for consistent benchmarks

	// Check if BTC genesis header exists, if not create it
	initHeader := btcKeeper.GetHeaderByHeight(ctx, 0)
	if initHeader == nil {
		// Initialize BTC genesis header using signet genesis  
		genesisHeader, err := app.SignetBtcHeaderGenesis(babylonApp.AppCodec())
		require.NoError(b, err)
		
		// Check if base header already exists to avoid panic
		if btcKeeper.GetBaseBTCHeader(ctx) == nil {
			// Set the base BTC header (genesis at height 0)
			genesisHeader.Height = 0
			btcKeeper.SetBaseBTCHeader(ctx, *genesisHeader)
		}
		
		// Retrieve the now-initialized header
		initHeader = btcKeeper.GetHeaderByHeight(ctx, 0)
		require.NotNil(b, initHeader, "BTC genesis header should be initialized")
	}

	// Generate chain from initial header
	randomChain := datagen.NewBTCHeaderChainFromParentInfo(r, initHeader, chainLength)
	err := btcKeeper.InsertHeadersWithHookAndEvents(ctx, randomChain.ChainToBytes())
	require.NoError(b, err)

	// Verify tip
	tip := btcKeeper.GetTipInfo(ctx)
	chainTipInfo := randomChain.GetTipInfo()
	require.Equal(b, tip.Height, chainTipInfo.Height)

	return randomChain
}

func setupBTCHeadersConsumersAndChannels(b *testing.B, app *app.BabylonApp, ctx sdk.Context, consumers, channelsPerConsumer int, simulateCacheHits bool) []channeltypes.IdentifiedChannel {
	b.Helper()

	var allChannels []channeltypes.IdentifiedChannel

	for i := 0; i < consumers; i++ {
		consumerID := fmt.Sprintf("consumer-%d", i)

		// Setup consumer
		setupBTCHeadersConsumer(b, app, ctx, consumerID, i)

		// If simulating cache hits, set up some consumers to share the same headers
		// by setting similar last sent segments
		if simulateCacheHits && i > 0 && i%3 == 0 {
			// Copy last sent segment from previous consumer to simulate shared headers
			prevConsumerID := fmt.Sprintf("consumer-%d", i-1)
			if segment := app.ZoneConciergeKeeper.GetBSNLastSentSegment(ctx, prevConsumerID); segment != nil {
				app.ZoneConciergeKeeper.SetBSNLastSentSegment(ctx, consumerID, segment)
			}
		}

		// Setup IBC infrastructure
		channels := setupConsumerIBCInfrastructure(b, app, ctx, consumerID, i, channelsPerConsumer)
		allChannels = append(allChannels, channels...)
	}

	return allChannels
}

func setupBTCHeadersConsumer(b *testing.B, app *app.BabylonApp, ctx sdk.Context, consumerID string, index int) {
	b.Helper()

	// Register consumer in BTC staking consumer keeper
	consumerRegister := &bsctypes.ConsumerRegister{
		ConsumerId:          consumerID,
		ConsumerName:        fmt.Sprintf("BTC Headers Benchmark Consumer %d", index),
		ConsumerDescription: fmt.Sprintf("Consumer for BTC headers benchmark testing %d", index),
		ConsumerMetadata: &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
			CosmosConsumerMetadata: &bsctypes.CosmosConsumerMetadata{
				ChannelId: fmt.Sprintf("channel-%d-0", index), // Use first channel as primary
			},
		},
	}
	err := app.BTCStkConsumerKeeper.RegisterConsumer(ctx, consumerRegister)
	require.NoError(b, err)
}

func setupConsumerIBCInfrastructure(b *testing.B, app *app.BabylonApp, ctx sdk.Context, consumerID string, index, channelsPerConsumer int) []channeltypes.IdentifiedChannel {
	b.Helper()

	var channels []channeltypes.IdentifiedChannel

	// Setup IBC client
	app.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID, &ibctmtypes.ClientState{})

	// Setup connection
	connectionID := fmt.Sprintf("connection-%d", index)
	app.IBCKeeper.ConnectionKeeper.SetConnection(
		ctx, connectionID, connectiontypes.ConnectionEnd{
			ClientId: consumerID,
		},
	)

	// Setup multiple channels for this consumer
	for j := 0; j < channelsPerConsumer; j++ {
		channelID := fmt.Sprintf("channel-%d-%d", index, j)
		portID := types.PortID

		// Setup IBC channel
		app.IBCKeeper.ChannelKeeper.SetChannel(
			ctx, portID, channelID, channeltypes.Channel{
				State:          channeltypes.OPEN,
				ConnectionHops: []string{connectionID},
			},
		)

		// Create IdentifiedChannel for the benchmark
		identifiedChannel := channeltypes.IdentifiedChannel{
			State:          channeltypes.OPEN,
			Ordering:       channeltypes.UNORDERED,
			Counterparty:   channeltypes.Counterparty{},
			ConnectionHops: []string{connectionID},
			Version:        "1",
			PortId:         portID,
			ChannelId:      channelID,
		}
		channels = append(channels, identifiedChannel)
	}

	return channels
}
