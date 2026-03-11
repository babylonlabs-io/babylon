package v2_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	v2 "github.com/babylonlabs-io/babylon/v4/x/btcstaking/migrations/v2"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func TestMigrateStore(t *testing.T) {
	btcstakingKeeper, ctx, cdc, kvStore := keepertest.BTCStakingKeeperWithStoreService(t, nil, nil, nil)

	heightToVersionMap := &types.HeightToVersionMap{
		Pairs: []*types.HeightVersionPair{
			{StartHeight: 100, Version: 0},
			{StartHeight: 200, Version: 1},
			{StartHeight: 300, Version: 2},
		},
	}

	bz := cdc.MustMarshal(heightToVersionMap)
	err := kvStore.Set(v2.OldHeightToVersionMapKey, bz)
	require.NoError(t, err)

	oldBz, err := kvStore.Get(v2.OldHeightToVersionMapKey)
	require.NoError(t, err)
	require.NotNil(t, oldBz)

	newMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.Nil(t, newMap)

	testFpAddr := datagen.GenRandomAddress()
	err = btcstakingKeeper.SetFpBbnAddr(ctx, testFpAddr)
	require.NoError(t, err)

	hasFpAddr, err := btcstakingKeeper.HasFpRegistered(ctx, testFpAddr)
	require.NoError(t, err)
	require.True(t, hasFpAddr)

	err = v2.MigrateStore(ctx, cdc, kvStore, btcstakingKeeper)
	require.NoError(t, err)

	oldBz, err = kvStore.Get(v2.OldHeightToVersionMapKey)
	require.NoError(t, err)
	require.Nil(t, oldBz)

	migratedMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.NotNil(t, migratedMap)
	require.Equal(t, len(heightToVersionMap.Pairs), len(migratedMap.Pairs))

	for i, pair := range heightToVersionMap.Pairs {
		require.Equal(t, pair.StartHeight, migratedMap.Pairs[i].StartHeight)
		require.Equal(t, pair.Version, migratedMap.Pairs[i].Version)
	}

	hasFpAddrAfter, err := btcstakingKeeper.HasFpRegistered(ctx, testFpAddr)
	require.NoError(t, err)
	require.True(t, hasFpAddrAfter)
}

func TestMigrateStore_WithMigrator(t *testing.T) {
	btcstakingKeeper, ctx, cdc, kvStore := keepertest.BTCStakingKeeperWithStoreService(t, nil, nil, nil)

	heightToVersionMap := &types.HeightToVersionMap{
		Pairs: []*types.HeightVersionPair{
			{StartHeight: 500, Version: 0},
		},
	}
	bz := cdc.MustMarshal(heightToVersionMap)
	err := kvStore.Set(v2.OldHeightToVersionMapKey, bz)
	require.NoError(t, err)

	testFpAddr := datagen.GenRandomAddress()
	err = btcstakingKeeper.SetFpBbnAddr(ctx, testFpAddr)
	require.NoError(t, err)

	hasFpAddr, err := btcstakingKeeper.HasFpRegistered(ctx, testFpAddr)
	require.NoError(t, err)
	require.True(t, hasFpAddr)

	m := keeper.NewMigrator(*btcstakingKeeper)
	err = m.Migrate1to2(ctx)
	require.NoError(t, err)

	migratedMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.NotNil(t, migratedMap)
	require.Equal(t, len(heightToVersionMap.Pairs), len(migratedMap.Pairs))
	require.Equal(t, heightToVersionMap.Pairs[0].StartHeight, migratedMap.Pairs[0].StartHeight)
	require.Equal(t, heightToVersionMap.Pairs[0].Version, migratedMap.Pairs[0].Version)

	oldBz, err := kvStore.Get(v2.OldHeightToVersionMapKey)
	require.NoError(t, err)
	require.Nil(t, oldBz)

	hasFpAddrAfter, err := btcstakingKeeper.HasFpRegistered(ctx, testFpAddr)
	require.NoError(t, err)
	require.True(t, hasFpAddrAfter)
}

func TestMigrateStore_OldKeyEmpty_RebuildFromParams(t *testing.T) {
	btcstakingKeeper, ctx, cdc, kvStore := keepertest.BTCStakingKeeperWithStoreService(t, nil, nil, nil)

	p0 := types.DefaultParams()
	p0.BtcActivationHeight = 100
	err := btcstakingKeeper.SetParams(ctx, p0)
	require.NoError(t, err)

	p1 := types.DefaultParams()
	p1.BtcActivationHeight = 200
	err = btcstakingKeeper.SetParams(ctx, p1)
	require.NoError(t, err)

	// Delete the HeightToVersionMap via kvStore to simulate empty old key scenario
	err = kvStore.Delete(types.HeightToVersionMapKey)
	require.NoError(t, err)
	require.Nil(t, btcstakingKeeper.GetHeightToVersionMap(ctx))

	// Verify old key is not set
	oldBz, err := kvStore.Get(v2.OldHeightToVersionMapKey)
	require.NoError(t, err)
	require.Nil(t, oldBz)

	err = v2.MigrateStore(ctx, cdc, kvStore, btcstakingKeeper)
	require.NoError(t, err)

	rebuiltMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.NotNil(t, rebuiltMap)
	require.Equal(t, 2, len(rebuiltMap.Pairs))
	require.Equal(t, uint64(100), rebuiltMap.Pairs[0].StartHeight)
	require.Equal(t, uint32(0), rebuiltMap.Pairs[0].Version)
	require.Equal(t, uint64(200), rebuiltMap.Pairs[1].StartHeight)
	require.Equal(t, uint32(1), rebuiltMap.Pairs[1].Version)
}

func TestMigrateStore_WithMigrator_OldKeyEmpty(t *testing.T) {
	btcstakingKeeper, ctx, _, kvStore := keepertest.BTCStakingKeeperWithStoreService(t, nil, nil, nil)

	p0 := types.DefaultParams()
	p0.BtcActivationHeight = 100
	err := btcstakingKeeper.SetParams(ctx, p0)
	require.NoError(t, err)

	p1 := types.DefaultParams()
	p1.BtcActivationHeight = 200
	err = btcstakingKeeper.SetParams(ctx, p1)
	require.NoError(t, err)

	// Delete the HeightToVersionMap via kvStore to simulate empty old key scenario
	err = kvStore.Delete(types.HeightToVersionMapKey)
	require.NoError(t, err)
	require.Nil(t, btcstakingKeeper.GetHeightToVersionMap(ctx))

	m := keeper.NewMigrator(*btcstakingKeeper)
	err = m.Migrate1to2(ctx)
	require.NoError(t, err)

	rebuiltMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.NotNil(t, rebuiltMap)
	require.Equal(t, 2, len(rebuiltMap.Pairs))
	require.Equal(t, uint64(100), rebuiltMap.Pairs[0].StartHeight)
	require.Equal(t, uint32(0), rebuiltMap.Pairs[0].Version)
	require.Equal(t, uint64(200), rebuiltMap.Pairs[1].StartHeight)
	require.Equal(t, uint32(1), rebuiltMap.Pairs[1].Version)
}
