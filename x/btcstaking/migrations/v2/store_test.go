package v2_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	v2 "github.com/babylonlabs-io/babylon/v4/x/btcstaking/migrations/v2"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func TestMigrateStore(t *testing.T) {
	btcstakingKeeper, ctx, cdc, kvStore := keepertest.BTCStakingKeeperWithStoreService(t, nil, nil, nil)

	// Create test HeightToVersionMap
	heightToVersionMap := &types.HeightToVersionMap{
		Pairs: []*types.HeightVersionPair{
			{StartHeight: 100, Version: 0},
			{StartHeight: 200, Version: 1},
			{StartHeight: 300, Version: 2},
		},
	}

	// Manually set HeightToVersionMap using old key (0x10 = decimal 16)
	bz := cdc.MustMarshal(heightToVersionMap)
	err := kvStore.Set(v2.OldHeightToVersionMapKey, bz)
	require.NoError(t, err)

	// Verify old key exists
	oldBz, err := kvStore.Get(v2.OldHeightToVersionMapKey)
	require.NoError(t, err)
	require.NotNil(t, oldBz)

	// Verify new key doesn't exist yet
	newMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.Nil(t, newMap)

	// Perform migration
	err = v2.MigrateStore(ctx, cdc, kvStore, btcstakingKeeper)
	require.NoError(t, err)

	// Verify old key is deleted
	oldBz, err = kvStore.Get(v2.OldHeightToVersionMapKey)
	require.NoError(t, err)
	require.Nil(t, oldBz)

	// Verify data migrated to new key
	migratedMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.NotNil(t, migratedMap)
	require.Equal(t, len(heightToVersionMap.Pairs), len(migratedMap.Pairs))

	for i, pair := range heightToVersionMap.Pairs {
		require.Equal(t, pair.StartHeight, migratedMap.Pairs[i].StartHeight)
		require.Equal(t, pair.Version, migratedMap.Pairs[i].Version)
	}
}

func TestMigrateStore_WithMigrator(t *testing.T) {
	btcstakingKeeper, ctx, cdc, kvStore := keepertest.BTCStakingKeeperWithStoreService(t, nil, nil, nil)

	// Create test HeightToVersionMap and set it using the old key
	heightToVersionMap := &types.HeightToVersionMap{
		Pairs: []*types.HeightVersionPair{
			{StartHeight: 500, Version: 0},
		},
	}
	bz := cdc.MustMarshal(heightToVersionMap)
	err := kvStore.Set(v2.OldHeightToVersionMapKey, bz)
	require.NoError(t, err)

	// Perform migration through Migrator
	m := keeper.NewMigrator(*btcstakingKeeper)
	err = m.Migrate1to2(ctx)
	require.NoError(t, err)

	// Verify migration was successful
	migratedMap := btcstakingKeeper.GetHeightToVersionMap(ctx)
	require.NotNil(t, migratedMap)
	require.Equal(t, len(heightToVersionMap.Pairs), len(migratedMap.Pairs))
	require.Equal(t, heightToVersionMap.Pairs[0].StartHeight, migratedMap.Pairs[0].StartHeight)
	require.Equal(t, heightToVersionMap.Pairs[0].Version, migratedMap.Pairs[0].Version)

	// Verify old key is deleted
	oldBz, err := kvStore.Get(v2.OldHeightToVersionMapKey)
	require.NoError(t, err)
	require.Nil(t, oldBz)
}
