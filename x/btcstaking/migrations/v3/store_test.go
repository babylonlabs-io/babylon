package v3_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"

	"github.com/stretchr/testify/require"
)

func TestMigrateStore(t *testing.T) {
	var (
		r                     = rand.New(rand.NewSource(time.Now().UnixNano()))
		storeKey              = storetypes.NewKVStoreKey(types.StoreKey)
		btcStakingKeeper, ctx = keepertest.BTCStakingKeeperWithStoreKey(t, storeKey, nil, nil, nil, nil)
		nAllowedTxHashes      = 5
		nMultiStakingTxHashes = 3
	)

	store := ctx.KVStore(storeKey)

	// Populate AllowedStakingTxHashesKeySet with test data
	for i := 0; i < nAllowedTxHashes; i++ {
		txHash := datagen.GenRandomHexStr(r, 64)
		txHashBytes := []byte(txHash)
		err := btcStakingKeeper.AllowedStakingTxHashesKeySet.Set(ctx, txHashBytes)
		require.NoError(t, err)
	}

	// Populate allowedMultiStakingTxHashesKeySet with test data directly via store
	multiStakingStore := prefix.NewStore(store, types.AllowedMultiStakingTxHashesKey.Bytes())
	for i := 0; i < nMultiStakingTxHashes; i++ {
		txHash := datagen.GenRandomHexStr(r, 64)
		txHashBytes := []byte(txHash)
		multiStakingStore.Set(txHashBytes, []byte{})
	}

	// Verify data exists before migration
	allowedTxHashCount := 0
	iter1, err := btcStakingKeeper.AllowedStakingTxHashesKeySet.Iterate(ctx, nil)
	require.NoError(t, err)
	for ; iter1.Valid(); iter1.Next() {
		allowedTxHashCount++
	}
	iter1.Close()
	require.Equal(t, nAllowedTxHashes, allowedTxHashCount)

	// Count multi-staking tx hashes using store iterator
	multiStakingTxHashCount := 0
	iter2 := multiStakingStore.Iterator(nil, nil)
	for ; iter2.Valid(); iter2.Next() {
		multiStakingTxHashCount++
	}
	iter2.Close()
	require.Equal(t, nMultiStakingTxHashes, multiStakingTxHashCount)

	// Perform migration
	m := keeper.NewMigrator(*btcStakingKeeper)
	require.NoError(t, m.Migrate2to3(ctx))

	// Verify data is cleared after migration
	allowedTxHashCountAfter := 0
	iter3, err := btcStakingKeeper.AllowedStakingTxHashesKeySet.Iterate(ctx, nil)
	require.NoError(t, err)
	for ; iter3.Valid(); iter3.Next() {
		allowedTxHashCountAfter++
	}
	iter3.Close()
	require.Equal(t, 0, allowedTxHashCountAfter)

	// Count multi-staking tx hashes after migration
	multiStakingTxHashCountAfter := 0
	iter4 := multiStakingStore.Iterator(nil, nil)
	for ; iter4.Valid(); iter4.Next() {
		multiStakingTxHashCountAfter++
	}
	iter4.Close()
	require.Equal(t, 0, multiStakingTxHashCountAfter)
}
