package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/stretchr/testify/require"
)

const consumerID = "test-consumerID"

func FuzzCanonicalChainIndexer(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		// simulate a random number of blocks
		numHeaders := datagen.RandomInt(r, 100) + 1
		headers := SimulateNewHeaders(ctx, r, &zcKeeper, consumerID, 0, numHeaders)

		// check if the canonical chain index is correct or not
		for i := uint64(0); i < numHeaders; i++ {
			header, err := zcKeeper.GetHeader(ctx, consumerID, i)
			require.NoError(t, err)
			require.NotNil(t, header)
			require.Equal(t, consumerID, header.ConsumerId)
			require.Equal(t, i, header.Height)
			require.Equal(t, headers[i].Header.AppHash, header.Hash)
		}

		// check if the chain info is updated or not
		chainInfo, err := zcKeeper.GetChainInfo(ctx, consumerID)
		require.NoError(t, err)
		require.NotNil(t, chainInfo.LatestHeader)
		require.Equal(t, consumerID, chainInfo.LatestHeader.ConsumerId)
		require.Equal(t, numHeaders-1, chainInfo.LatestHeader.Height)
		require.Equal(t, headers[numHeaders-1].Header.AppHash, chainInfo.LatestHeader.Hash)
	})
}

func FuzzFindClosestHeader(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		// no header at the moment, FindClosestHeader invocation should give error
		_, err := zcKeeper.FindClosestHeader(ctx, consumerID, 100)
		require.Error(t, err)

		// simulate a random number of blocks
		numHeaders := datagen.RandomInt(r, 100) + 1
		headers := SimulateNewHeaders(ctx, r, &zcKeeper, consumerID, 0, numHeaders)

		header, err := zcKeeper.FindClosestHeader(ctx, consumerID, numHeaders)
		require.NoError(t, err)
		require.Equal(t, headers[len(headers)-1].Header.AppHash, header.Hash)

		// skip a non-zero number of headers in between, in order to create a gap of non-timestamped headers
		gap := datagen.RandomInt(r, 10) + 1

		// simulate a random number of blocks
		// where the new batch of headers has a gap with the previous batch
		SimulateNewHeaders(ctx, r, &zcKeeper, consumerID, numHeaders+gap+1, numHeaders)

		// get a random height that is in this gap
		randomHeightInGap := datagen.RandomInt(r, int(gap+1)) + numHeaders
		// find the closest header with the given randomHeightInGap
		header, err = zcKeeper.FindClosestHeader(ctx, consumerID, randomHeightInGap)
		require.NoError(t, err)
		// the header should be the same as the last header in the last batch
		require.Equal(t, headers[len(headers)-1].Header.AppHash, header.Hash)
	})
}
