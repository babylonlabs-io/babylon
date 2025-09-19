package v2_test

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
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"

	"github.com/stretchr/testify/require"
)

func TestMigrateStore(t *testing.T) {
	var (
		r            = rand.New(rand.NewSource(time.Now().UnixNano()))
		db           = dbm.NewMemDB()
		stateStore   = store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
		storeKey     = storetypes.NewKVStoreKey(types.StoreKey)
		storeService = runtime.NewKVStoreService(storeKey)
		cdc          = appparams.DefaultEncodingConfig().Codec
		fKeeper, ctx = keepertest.FinalityKeeperWithStoreKey(t, db, stateStore, storeKey, nil, nil, nil, nil)
		fpCount      = 5
		commitCount  = datagen.RandomIntOtherThan(r, 0, 100)
	)

	// setup store with 5 fps and some PubRandCommits
	fpBtcPKs := make([]*bbn.BIP340PubKey, 0, fpCount)
	for range fpCount {
		fpBtcPK, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		fpBtcPKs = append(fpBtcPKs, fpBtcPK)
		populateCommits(t, ctx, storeService.OpenKVStore(ctx), fKeeper, fpBtcPK, int(commitCount))
		// double check that there's no index for the PubRandCommits
		_, err = getIndex(storeService.OpenKVStore(ctx), cdc, fpBtcPK)
		require.Error(t, err)
		require.ErrorContains(t, err, collections.ErrNotFound.Error())
	}

	// Perform migration
	m := keeper.NewMigrator(*fKeeper)
	require.NoError(t, m.Migrate1to2(ctx))

	// Check migration was successful
	// PubRandCommit indexes should be present
	expStartHeight := make([]uint64, 0, commitCount)
	for i := uint64(0); i < commitCount; i++ {
		startHeight := i * 10
		expStartHeight = append(expStartHeight, startHeight)
	}

	for _, fpBtcPK := range fpBtcPKs {
		idx, err := getIndex(storeService.OpenKVStore(ctx), cdc, fpBtcPK)
		require.NoError(t, err)
		require.Equal(t, expStartHeight, idx.Heights)
	}
}

func populateCommits(t *testing.T, ctx context.Context, s corestore.KVStore, fKeeper *keeper.Keeper, fpBtcPK *bbn.BIP340PubKey, n int) {
	for i := 0; i < n; i++ {
		startHeight := uint64(i * 10)
		commit := &types.PubRandCommit{
			StartHeight: startHeight,
			NumPubRand:  10,
			Commitment:  []byte(fmt.Sprintf("commit-%d", i)),
			EpochNum:    uint64(i),
		}

		err := fKeeper.SetPubRandCommit(ctx, fpBtcPK, commit)
		require.NoError(t, err)
	}
	// delete the index created on the SetPubRandCommit to test the migration
	err := deleteIndex(s, fpBtcPK)
	require.NoError(t, err)
}

func deleteIndex(store corestore.KVStore, fpBtcPK *bbn.BIP340PubKey) error {
	bytesKey, err := collections.EncodeKeyWithPrefix(types.PubRandCommitIndexKeyPrefix.Bytes(), collections.BytesKey, fpBtcPK.MustMarshal())
	if err != nil {
		return err
	}

	return store.Delete(bytesKey)
}

func getIndex(store corestore.KVStore, cdc codec.BinaryCodec, fpBtcPK *bbn.BIP340PubKey) (types.PubRandCommitIndexValue, error) {
	bytesKey, err := collections.EncodeKeyWithPrefix(types.PubRandCommitIndexKeyPrefix.Bytes(), collections.BytesKey, fpBtcPK.MustMarshal())
	if err != nil {
		return types.PubRandCommitIndexValue{}, err
	}

	valueBz, err := store.Get(bytesKey)
	if err != nil {
		return types.PubRandCommitIndexValue{}, err
	}

	if valueBz == nil {
		return types.PubRandCommitIndexValue{}, collections.ErrNotFound
	}

	valueCdc := codec.CollValue[types.PubRandCommitIndexValue](cdc)
	return valueCdc.Decode(valueBz)
}
