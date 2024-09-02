package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/stretchr/testify/require"
)

func FuzzForkIndexer(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)
		consumerID := "test-consumerid"

		// invoke the hook a random number of times to simulate a random number of blocks
		numHeaders := datagen.RandomInt(r, 100) + 1
		numForkHeaders := datagen.RandomInt(r, 10) + 1
		_, forkHeaders := SimulateNewHeadersAndForks(ctx, r, &zcKeeper, consumerID, 0, numHeaders, numForkHeaders)

		// check if the fork is updated or not
		forks := zcKeeper.GetForks(ctx, consumerID, numHeaders-1)
		require.Equal(t, numForkHeaders, uint64(len(forks.Headers)))
		for i := range forks.Headers {
			require.Equal(t, numHeaders-1, forks.Headers[i].Height)
			require.Equal(t, forkHeaders[i].Header.AppHash, forks.Headers[i].Hash)
		}

		// check if the chain info is updated or not
		chainInfo, err := zcKeeper.GetChainInfo(ctx, consumerID)
		require.NoError(t, err)
		require.Equal(t, numForkHeaders, uint64(len(chainInfo.LatestForks.Headers)))
		for i := range forks.Headers {
			require.Equal(t, consumerID, chainInfo.LatestForks.Headers[i].ConsumerId)
			require.Equal(t, numHeaders-1, chainInfo.LatestForks.Headers[i].Height)
			require.Equal(t, forkHeaders[i].Header.AppHash, chainInfo.LatestForks.Headers[i].Hash)
		}
	})
}
