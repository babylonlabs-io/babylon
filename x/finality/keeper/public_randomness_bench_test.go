package keeper_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/collections"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"

	"github.com/stretchr/testify/require"
)

// BenchmarkCompareIndexVsReverse benchmarks the GetPubRandCommitForHeight
// function for the 2 possible paths:
// - using an index of the PubRandCommit startHeights
// - using the reverse iterator (fallback & legacy method)
func BenchmarkCompareIndexVsReverse(b *testing.B) {
	benchmarkGetPubRandCommit(b, 100)
	benchmarkGetPubRandCommit(b, 1000)
	benchmarkGetPubRandCommit(b, 10000)
}

func benchmarkGetPubRandCommit(b *testing.B, numRecords int) {
	b.Helper()
	var (
		r            = rand.New(rand.NewSource(time.Now().UnixNano()))
		db           = dbm.NewMemDB()
		stateStore   = store.NewCommitMultiStore(db, log.NewTestLogger(b), storemetrics.NewNoOpMetrics())
		storeKey     = storetypes.NewKVStoreKey(types.StoreKey)
		storeService = runtime.NewKVStoreService(storeKey)
		fKeeper, ctx = keepertest.FinalityKeeperWithStoreKey(b, db, stateStore, storeKey, nil, nil, nil)
		fpBtcPK, err = datagen.GenRandomBIP340PubKey(r)
	)
	require.NoError(b, err)
	populateCommits(b, ctx, fKeeper, fpBtcPK, numRecords)

	targetHeight := uint64((numRecords-1)*10 + 5)

	b.Run(fmt.Sprintf("IndexPath_%d", numRecords), func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := fKeeper.GetPubRandCommitForHeight(ctx, fpBtcPK, targetHeight)
			require.NoError(b, err)
		}
	})

	// Remove index to force reverse scan
	require.NoError(b, deleteIndex(ctx, storeService.OpenKVStore(ctx), fpBtcPK))

	b.Run(fmt.Sprintf("ReversePath_%d", numRecords), func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := fKeeper.GetPubRandCommitForHeight(ctx, fpBtcPK, targetHeight)
			require.NoError(b, err)
		}
	})
}

func populateCommits(b *testing.B, ctx context.Context, fKeeper *keeper.Keeper, fpBtcPK *bbn.BIP340PubKey, n int) {
	for i := 0; i < n; i++ {
		startHeight := uint64(i * 10)
		commit := &types.PubRandCommit{
			StartHeight: startHeight,
			NumPubRand:  10,
			Commitment:  []byte(fmt.Sprintf("commit-%d", i)),
			EpochNum:    uint64(i),
		}

		err := fKeeper.SetPubRandCommit(ctx, fpBtcPK, commit)
		require.NoError(b, err)
	}
}

func deleteIndex(ctx context.Context, store corestore.KVStore, fpBtcPK *bbn.BIP340PubKey) error {
	bytesKey, err := collections.EncodeKeyWithPrefix(types.PubRandCommitIndexKeyPrefix.Bytes(), collections.BytesKey, fpBtcPK.MustMarshal())
	if err != nil {
		return err
	}

	return store.Delete(bytesKey)
}
