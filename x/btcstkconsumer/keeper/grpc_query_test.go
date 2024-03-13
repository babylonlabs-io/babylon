package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/app"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/babylon/testutil/datagen"
	zctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

type chainRegister struct {
	chainID string
}

func FuzzChainRegistryList(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// invoke the chain registration a random number of times with random chain IDs
		numRegistrations := datagen.RandomInt(r, 100) + 1
		var allChainIDs []string
		for i := uint64(0); i < numRegistrations; i++ {
			var chainID = datagen.GenRandomHexStr(r, 30)
			allChainIDs = append(allChainIDs, chainID)

			zcKeeper.SetChainRegister(ctx, &zctypes.ChainRegister{
				ChainId:   chainID,
				ChainName: datagen.GenRandomHexStr(r, 5),
			})
		}

		limit := datagen.RandomInt(r, len(allChainIDs)) + 1

		// Query to get actual chain IDs
		resp, err := zcKeeper.ChainRegistryList(ctx, &zctypes.QueryChainRegistryListRequest{
			Pagination: &query.PageRequest{
				Limit: limit,
			},
		})
		require.NoError(t, err)
		actualChainIDs := resp.ChainIds

		require.Equal(t, limit, uint64(len(actualChainIDs)))
		for i := uint64(0); i < limit; i++ {
			require.Contains(t, allChainIDs, actualChainIDs[i])
		}
	})
}

func FuzzChainsRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		var (
			chainsRegister []chainRegister
			chainIDs       []string
		)
		// invoke the chain registration a random number of times with random chain IDs
		numChains := datagen.RandomInt(r, 100) + 1
		for i := uint64(0); i < numChains; i++ {
			chainID := datagen.GenRandomHexStr(r, 30)

			chainIDs = append(chainIDs, chainID)
			chainsRegister = append(chainsRegister, chainRegister{
				chainID: chainID,
			})

			zcKeeper.SetChainRegister(ctx, &zctypes.ChainRegister{
				ChainId:   chainID,
				ChainName: datagen.GenRandomHexStr(r, 5),
			})

		}

		resp, err := zcKeeper.ChainsRegistry(ctx, &zctypes.QueryChainsRegistryRequest{
			ChainIds: chainIDs,
		})
		require.NoError(t, err)

		for i, respData := range resp.ChainsRegister {
			require.Equal(t, chainsRegister[i].chainID, respData.ChainId)
		}
	})
}
