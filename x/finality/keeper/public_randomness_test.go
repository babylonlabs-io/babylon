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
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"

	"github.com/stretchr/testify/require"
)

func TestGetPubRandCommitForHeight(t *testing.T) {
	tests := []struct {
		name               string
		height             uint64
		valid              bool
		expectedCommitment string
		mutateStore        func(context.Context, corestore.KVStoreService, *keeper.Keeper, *bbn.BIP340PubKey)
		errMsg             string
	}{
		{
			name:               "height within first commit",
			height:             5,
			valid:              true,
			expectedCommitment: "commit-0",
		},
		{
			name:               "height at start of second commit",
			height:             10,
			valid:              true,
			expectedCommitment: "commit-1",
		},
		{
			name:               "height at end of last commit",
			height:             29,
			valid:              true,
			expectedCommitment: "commit-2",
		},
		{
			name:               "height before first commit",
			height:             0,
			valid:              true,
			expectedCommitment: "commit-0",
		},
		{
			name:   "height after all commits",
			height: 30,
			valid:  false,
		},
		{
			name:   "empty index",
			height: 5,
			valid:  false,
			mutateStore: func(ctx context.Context, ss corestore.KVStoreService, k *keeper.Keeper, fpBtcPK *bbn.BIP340PubKey) {
				require.NoError(t, deleteIndex(ss.OpenKVStore(ctx), fpBtcPK))
			},
			errMsg: collections.ErrNotFound.Error(),
		},
		{
			name:   "commit data missing in store",
			height: 15,
			valid:  false,
			mutateStore: func(ctx context.Context, ss corestore.KVStoreService, k *keeper.Keeper, fpBtcPK *bbn.BIP340PubKey) {
				// Delete raw commit key for startHeight 10
				storeAdapter := runtime.KVStoreAdapter(ss.OpenKVStore(ctx))
				pubRandStore := prefix.NewStore(storeAdapter, types.PubRandCommitKey)
				fpStore := prefix.NewStore(pubRandStore, fpBtcPK.MustMarshal())
				fpStore.Delete(sdk.Uint64ToBigEndian(10))
			},
			errMsg: "public randomness is not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			fpBtcPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)
			// Setup: Add 3 commits [0-9], [10-19], [20-29]
			ctx, storeService, k := setupTest(t, fpBtcPK)
			if tc.mutateStore != nil {
				tc.mutateStore(ctx, storeService, k, fpBtcPK)
			}

			commit, err := k.GetPubRandCommitForHeight(ctx, fpBtcPK, tc.height)

			if tc.valid {
				require.NoError(t, err)
				require.NotNil(t, commit)
				require.Equal(t, []byte(tc.expectedCommitment), commit.Commitment)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
			require.Nil(t, commit)
		})
	}
}

func setupTest(t *testing.T, fpBtcPK *bbn.BIP340PubKey) (context.Context, corestore.KVStoreService, *keeper.Keeper) {
	var (
		db           = dbm.NewMemDB()
		storeKey     = storetypes.NewKVStoreKey(types.StoreKey)
		stateStore   = store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
		storeService = runtime.NewKVStoreService(storeKey)
		fKeeper, ctx = keepertest.FinalityKeeperWithStoreKey(t, db, stateStore, storeKey, nil, nil, nil, nil)
	)

	// Setup: Add 3 commits [0-9], [10-19], [20-29]
	for i := 0; i < 3; i++ {
		commit := &types.PubRandCommit{
			StartHeight: uint64(i * 10),
			NumPubRand:  10,
			Commitment:  []byte(fmt.Sprintf("commit-%d", i)),
			EpochNum:    uint64(i),
		}
		err := fKeeper.SetPubRandCommit(ctx, fpBtcPK, commit)
		require.NoError(t, err)
	}
	return ctx, storeService, fKeeper
}

func deleteIndex(store corestore.KVStore, fpBtcPK *bbn.BIP340PubKey) error {
	bytesKey, err := collections.EncodeKeyWithPrefix(types.PubRandCommitIndexKeyPrefix.Bytes(), collections.BytesKey, fpBtcPK.MustMarshal())
	if err != nil {
		return err
	}

	return store.Delete(bytesKey)
}
