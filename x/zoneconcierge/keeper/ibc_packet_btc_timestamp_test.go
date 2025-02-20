package keeper_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	btclckeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
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

func FuzzGetHeadersToBroadcast(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		btclcKeeper := babylonApp.BTCLightClientKeeper
		ctx := babylonApp.NewContext(false)

		// insert initial btc headers and test header selection
		wValue := babylonApp.BtcCheckpointKeeper.GetParams(ctx).CheckpointFinalizationTimeout
		chainLength := uint32(datagen.RandomInt(r, 10)) + wValue
		genRandomChain(t, r, &btclcKeeper, ctx, 0, chainLength)
		headers := zcKeeper.GetHeadersToBroadcast(ctx)
		require.Len(t, headers, int(wValue)+1)
		zcKeeper.SetLastSentSegment(ctx, &types.BTCChainSegment{
			BtcHeaders: headers,
		})

		// verify last segment matches the last w+1 headers from tip
		btcTip := btclcKeeper.GetTipInfo(ctx)
		lastSegment := zcKeeper.GetLastSentSegment(ctx)
		require.Len(t, lastSegment.BtcHeaders, int(wValue)+1)
		for i := range lastSegment.BtcHeaders {
			require.Equal(t, btclcKeeper.GetHeaderByHeight(ctx, btcTip.Height-wValue+uint32(i)), lastSegment.BtcHeaders[i])
		}

		// insert additional headers and test header selection
		chainLength = uint32(datagen.RandomInt(r, 10)) + 1
		genRandomChain(t, r, &btclcKeeper, ctx, btcTip.Height, chainLength)
		headers = zcKeeper.GetHeadersToBroadcast(ctx)
		require.Len(t, headers, int(chainLength))
		zcKeeper.SetLastSentSegment(ctx, &types.BTCChainSegment{
			BtcHeaders: headers,
		})

		// verify last segment is since the header after the last tip
		lastSegment = zcKeeper.GetLastSentSegment(ctx)
		require.Len(t, lastSegment.BtcHeaders, int(chainLength))
		for i := range lastSegment.BtcHeaders {
			require.Equal(t, btclcKeeper.GetHeaderByHeight(ctx, uint32(i)+btcTip.Height+1), lastSegment.BtcHeaders[i])
		}

		// store current state for reorg test
		btcTip = btclcKeeper.GetTipInfo(ctx)
		lastSegmentLength := uint32(len(lastSegment.BtcHeaders))

		// create and insert a fork chain
		// NOTE: it's possible that the last segment is totally reverted. We want to be resilient against
		// this, by sending the BTC headers since the last reorg point
		reorgPoint := uint32(datagen.RandomInt(r, int(btcTip.Height)))
		revertedChainLength := btcTip.Height - reorgPoint
		// ensure fork is longer to trigger reorg
		forkChainLength := revertedChainLength + uint32(datagen.RandomInt(r, 10)) + 1
		genRandomChain(t, r, &btclcKeeper, ctx, reorgPoint, forkChainLength)
		headers = zcKeeper.GetHeadersToBroadcast(ctx)
		zcKeeper.SetLastSentSegment(ctx, &types.BTCChainSegment{
			BtcHeaders: headers,
		})

		// verify last segment is since the last reorg point
		btcTip = btclcKeeper.GetTipInfo(ctx)
		lastSegment = zcKeeper.GetLastSentSegment(ctx)
		if revertedChainLength >= lastSegmentLength {
			// the entire last segment is reverted, the last w+1 BTC headers should be sent
			require.Len(t, lastSegment.BtcHeaders, int(wValue)+1)
			// assert the consistency of w+1 sent BTC headers
			for i := range lastSegment.BtcHeaders {
				expectedHeight := btcTip.Height - wValue + uint32(i)
				require.Equal(t, btclcKeeper.GetHeaderByHeight(ctx, expectedHeight), lastSegment.BtcHeaders[i])
			}
		} else {
			// only a subset headers of last segment are reverted, only the new fork should be sent
			require.Len(t, lastSegment.BtcHeaders, int(forkChainLength))
			// assert the consistency of the sent fork BTC headers
			for i := range lastSegment.BtcHeaders {
				expectedHeight := btcTip.Height - forkChainLength + 1 + uint32(i)
				require.Equal(t, btclcKeeper.GetHeaderByHeight(ctx, expectedHeight), lastSegment.BtcHeaders[i])
			}
		}
	})
}
