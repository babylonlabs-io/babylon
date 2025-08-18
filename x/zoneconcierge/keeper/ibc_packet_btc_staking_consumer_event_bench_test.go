package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btcstkTypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"
)

// BenchmarkBroadcastBTCStakingConsumerEvents benchmarks the BroadcastBTCStakingConsumerEvents function
// with different scales of consumers, channels, and events
func BenchmarkBroadcastBTCStakingConsumerEvents(b *testing.B) {
	benchmarkCases := []struct {
		name                string
		consumers           int
		channelsPerConsumer int
		eventsPerConsumer   int
	}{
		{"Small_10c_1ch_5e", 10, 1, 5},
		{"Small_10c_2ch_10e", 10, 2, 10},
		{"Medium_50c_2ch_20e", 50, 2, 20},
		{"Medium_50c_3ch_30e", 50, 3, 30},
		{"Large_100c_2ch_50e", 100, 2, 50},
		{"Large_100c_4ch_100e", 100, 4, 100},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			app, ctx, zcKeeper := setupTest(b)

			// Setup consumers, channels, and events
			setupBenchmarkConsumersAndChannels(b, app, ctx, bc.consumers, bc.channelsPerConsumer)
			setupBenchmarkEvents(b, app, ctx, bc.consumers, bc.eventsPerConsumer)
			consumerChannelMap, err := app.ZoneConciergeKeeper.GetConsumerChannelMap(ctx)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				err := zcKeeper.BroadcastBTCStakingConsumerEvents(ctx, consumerChannelMap)
				require.NoError(b, err)

				// Re-setup events for next iteration (since they get deleted after broadcast)
				if i < b.N-1 {
					b.StopTimer()
					setupBenchmarkEvents(b, app, ctx, bc.consumers, bc.eventsPerConsumer)
					b.StartTimer()
				}
			}
		})
	}
}

// Helper functions for benchmark setup

func setupTest(b *testing.B) (*app.BabylonApp, sdk.Context, *keeper.Keeper) {
	b.Helper()

	babylonApp := app.Setup(&testing.T{}, false) // Convert b to t for setup
	ctx := babylonApp.NewContext(false)
	zcKeeper := &babylonApp.ZoneConciergeKeeper

	return babylonApp, ctx, zcKeeper
}

func setupBenchmarkConsumersAndChannels(b *testing.B, app *app.BabylonApp, ctx sdk.Context, consumers, channelsPerConsumer int) {
	b.Helper()

	for i := 0; i < consumers; i++ {
		consumerID := fmt.Sprintf("consumer-%d", i)

		// Setup IBC client
		app.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID, &ibctmtypes.ClientState{})

		// Setup connection
		connectionID := fmt.Sprintf("connection-%d", i)
		app.IBCKeeper.ConnectionKeeper.SetConnection(
			ctx, connectionID, connectiontypes.ConnectionEnd{
				ClientId: consumerID,
			},
		)

		// Setup multiple channels for this consumer
		for j := 0; j < channelsPerConsumer; j++ {
			channelID := fmt.Sprintf("channel-%d-%d", i, j)
			portID := types.PortID

			// Setup IBC channel
			app.IBCKeeper.ChannelKeeper.SetChannel(
				ctx, portID, channelID, channeltypes.Channel{
					State:          channeltypes.OPEN,
					ConnectionHops: []string{connectionID},
				},
			)
		}

		// Register consumer
		consumerRegister := &bsctypes.ConsumerRegister{
			ConsumerId:          consumerID,
			ConsumerName:        fmt.Sprintf("Benchmark Consumer %d", i),
			ConsumerDescription: fmt.Sprintf("Consumer for benchmark testing %d", i),
			ConsumerMetadata: &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
				CosmosConsumerMetadata: &bsctypes.CosmosConsumerMetadata{
					ChannelId: fmt.Sprintf("channel-%d-0", i), // Use first channel as primary
				},
			},
		}
		err := app.BTCStkConsumerKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(b, err)
	}
}

func setupBenchmarkEvents(b *testing.B, app *app.BabylonApp, ctx sdk.Context, consumers, eventsPerConsumer int) {
	b.Helper()

	r := rand.New(rand.NewSource(54321)) // Fixed seed for consistent benchmarks

	for i := 0; i < consumers; i++ {
		consumerID := fmt.Sprintf("consumer-%d", i)

		// Create realistic BTC staking consumer events
		events := make([]*btcstkTypes.BTCStakingConsumerEvent, 0, eventsPerConsumer)

		for j := 0; j < eventsPerConsumer; j++ {
			var event *btcstkTypes.BTCStakingConsumerEvent

			// Create different types of events for variety
			eventType := j % 3
			switch eventType {
			case 0: // New Finality Provider event
				event = &btcstkTypes.BTCStakingConsumerEvent{
					Event: &btcstkTypes.BTCStakingConsumerEvent_NewFp{
						NewFp: &btcstkTypes.NewFinalityProvider{
							Addr:       datagen.GenRandomAccount().Address,
							BtcPkHex:   fmt.Sprintf("%064x", datagen.GenRandomByteArray(r, 32)),
							Commission: "0.05", // 5% commission
							BsnId:      consumerID,
						},
					},
				}
			case 1: // Active Delegation event
				event = &btcstkTypes.BTCStakingConsumerEvent{
					Event: &btcstkTypes.BTCStakingConsumerEvent_ActiveDel{
						ActiveDel: &btcstkTypes.ActiveBTCDelegation{
							StakerAddr:    datagen.GenRandomAccount().Address,
							BtcPkHex:      fmt.Sprintf("%064x", datagen.GenRandomByteArray(r, 32)),
							FpBtcPkList:   []string{fmt.Sprintf("%064x", datagen.GenRandomByteArray(r, 32))},
							StartHeight:   uint32(datagen.RandomInt(r, 1000000)),
							EndHeight:     uint32(datagen.RandomInt(r, 1000000) + 1000000),
							TotalSat:      datagen.RandomInt(r, 100000000), // up to 1 BTC in sats
							StakingTx:     datagen.GenRandomByteArray(r, 100),
							UnbondingTime: uint32(144), // ~1 day in blocks
							ParamsVersion: 1,
						},
					},
				}
			case 2: // Unbonded Delegation event
				event = &btcstkTypes.BTCStakingConsumerEvent{
					Event: &btcstkTypes.BTCStakingConsumerEvent_UnbondedDel{
						UnbondedDel: &btcstkTypes.UnbondedBTCDelegation{
							StakingTxHash:   fmt.Sprintf("%064x", datagen.GenRandomByteArray(r, 32)),
							UnbondingTxSig:  datagen.GenRandomByteArray(r, 64),
							StakeSpendingTx: datagen.GenRandomByteArray(r, 200),
						},
					},
				}
			}

			events = append(events, event)
		}

		// Add events to the BTC staking keeper using the proper API
		err := app.BTCStakingKeeper.AddBTCStakingConsumerEvents(ctx, consumerID, events)
		require.NoError(b, err)
	}
}
