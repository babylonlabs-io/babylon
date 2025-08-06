package keeper_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btclckeeper "github.com/babylonlabs-io/babylon/v3/x/btclightclient/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	znctypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"
)

func allFieldsEqual(a *btclctypes.BTCHeaderInfo, b *btclctypes.BTCHeaderInfo) bool {
	return a.Height == b.Height && a.Hash.Eq(b.Hash) && a.Header.Eq(b.Header) && a.Work.Equal(*b.Work)
}

// this function must not be used at difficulty adjustment boundaries, as then
// difficulty adjustment calculation will fail
func genRandomChain(
	t *testing.T,
	r *rand.Rand,
	k *btclckeeper.Keeper,
	ctx context.Context,
	initialHeight uint32,
	chainLength uint32,
) *datagen.BTCHeaderPartialChain { //nolint:unparam // randomChain is used for test setup
	initHeader := k.GetHeaderByHeight(ctx, initialHeight)
	randomChain := datagen.NewBTCHeaderChainFromParentInfo(
		r,
		initHeader,
		chainLength,
	)
	err := k.InsertHeadersWithHookAndEvents(ctx, randomChain.ChainToBytes())
	require.NoError(t, err)
	tip := k.GetTipInfo(ctx)
	randomChainTipInfo := randomChain.GetTipInfo()
	require.True(t, allFieldsEqual(tip, randomChainTipInfo))
	return randomChain
}

func InitCosmosConsumer(app *app.BabylonApp, ctx sdk.Context, consumerID string) {
	app.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID, &ibctmtypes.ClientState{})

	app.IBCKeeper.ConnectionKeeper.SetConnection(
		ctx, consumerID, connectiontypes.ConnectionEnd{
			ClientId: consumerID,
		},
	)

	app.IBCKeeper.ChannelKeeper.SetChannel(
		ctx, "zoneconcierge", consumerID, channeltypes.Channel{
			State:          channeltypes.OPEN,
			ConnectionHops: []string{consumerID},
		},
	)

	consumerRegister := types.ConsumerRegister{
		ConsumerId:          consumerID,
		ConsumerName:        consumerID,
		ConsumerDescription: "test consumer",
		ConsumerMetadata: &types.ConsumerRegister_CosmosConsumerMetadata{
			CosmosConsumerMetadata: &types.CosmosConsumerMetadata{
				ChannelId: "zoneconcierge",
			},
		},
	}
	app.BTCStkConsumerKeeper.RegisterConsumer(ctx, &consumerRegister)
}

func FuzzGetHeadersToBroadcast(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcK, btclightK := babylonApp.ZoneConciergeKeeper, babylonApp.BTCLightClientKeeper
		ctx := babylonApp.NewContext(false)

		consumerID := "consumer1"
		InitCosmosConsumer(babylonApp, ctx, consumerID)

		// insert a random number of BTC headers to BTC light client
		kValue := babylonApp.BtcCheckpointKeeper.GetParams(ctx).BtcConfirmationDepth
		chainLength := uint32(datagen.RandomInt(r, 10)) + kValue*2
		genRandomChain(
			t,
			r,
			&btclightK,
			ctx,
			0,
			chainLength,
		)

		// current tip
		btcTip := btclightK.GetTipInfo(ctx)

		// At this point last segment is still nil

		// assert the last segment is the last k+1 BTC headers (using confirmation depth)
		btcHeaders := zcK.GetHeadersToBroadcast(ctx, consumerID)
		require.Len(t, btcHeaders, int(kValue)+1)
		for i := range btcHeaders {
			require.Equal(t, btclightK.GetHeaderByHeight(ctx, btcTip.Height-kValue+uint32(i)), btcHeaders[i])
		}

		// generates a few blocks but not enough to surpass `k` and check that it should return k+1 blocks
		chainLength = uint32(datagen.RandomInt(r, int(kValue-2))) + 1
		genRandomChain(
			t,
			r,
			&btclightK,
			ctx,
			btcTip.Height,
			chainLength,
		)

		// updates the tip
		btcTip = btclightK.GetTipInfo(ctx)

		// checks that returns k+1 headers
		btcHeaders2 := zcK.GetHeadersToBroadcast(ctx, consumerID)
		require.Len(t, btcHeaders2, int(kValue)+1)
		for i := range btcHeaders2 {
			require.Equal(t, btclightK.GetHeaderByHeight(ctx, uint32(i)+btcTip.Height-kValue), btcHeaders2[i])
		}

		// set the first headers as last segment to test without nil segments
		zcK.SetBSNLastSentSegment(ctx, consumerID, &znctypes.BTCChainSegment{
			BtcHeaders: btcHeaders,
		})

		// gets the headers again to check with last segments set and init header exists
		btcHeaders3 := zcK.GetHeadersToBroadcast(ctx, consumerID)
		require.Len(t, btcHeaders3, int(chainLength))
		for i := range btcHeaders3 {
			require.Equal(t, btclightK.GetHeaderByHeight(ctx, uint32(i)+btcTip.Height-chainLength+1), btcHeaders3[i])
		}

		// sets to the last segment set
		zcK.SetBSNLastSentSegment(ctx, consumerID, &znctypes.BTCChainSegment{
			BtcHeaders: btcHeaders3,
		})

		// Large reorg of BTC headers happens, which we want to be resilient against
		// send the BTC headers of at least k deep
		reorgPoint := uint32(datagen.RandomInt(r, int(btcTip.Height-1)-len(btcHeaders3)))
		revertedChainLength := btcTip.Height - reorgPoint
		// the fork chain needs to be longer than the canonical one
		forkChainLength := revertedChainLength + uint32(datagen.RandomInt(r, 2)) + kValue
		genRandomChain(
			t,
			r,
			&btclightK,
			ctx,
			reorgPoint,
			forkChainLength,
		)

		btcTip = btclightK.GetTipInfo(ctx)

		btcHeaders4 := zcK.GetHeadersToBroadcast(ctx, consumerID)
		require.Len(t, btcHeaders4, int(kValue)+1)
		for i := range btcHeaders4 {
			require.Equal(t, btclightK.GetHeaderByHeight(ctx, uint32(i)+btcTip.Height-kValue), btcHeaders4[i])
		}
	})
}
