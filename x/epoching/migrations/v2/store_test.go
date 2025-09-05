package v2_test

import (
	"testing"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	v1types "github.com/babylonlabs-io/babylon/v4/x/epoching/migrations/v1"
	v2 "github.com/babylonlabs-io/babylon/v4/x/epoching/migrations/v2"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMigrateStore(t *testing.T) {
	epochingKeeper, ctx := keepertest.EpochingKeeper(t)

	// Test Case 1: Test direct MigrateStore function with v1 params
	t.Run("migration_v1_to_v2", func(t *testing.T) {
		// Create test store and codec
		storeKey := storetypes.NewKVStoreKey(types.StoreKey)
		db := dbm.NewMemDB()
		stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
		stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
		require.NoError(t, stateStore.LoadLatestVersion())

		registry := codectypes.NewInterfaceRegistry()
		types.RegisterInterfaces(registry)
		cryptocodec.RegisterInterfaces(registry)
		cdc := codec.NewProtoCodec(registry)

		testCtx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
		testCtx = testCtx.WithHeaderInfo(header.Info{})

		kvStore := stateStore.GetKVStore(storeKey)

		// Manually set v1 params in store (EpochInterval only)
		v1Params := v1types.Params{
			EpochInterval: 100, // Custom test value
		}

		v1Bz, err := cdc.Marshal(&v1Params)
		require.NoError(t, err)
		kvStore.Set(types.ParamsKey, v1Bz)

		// Call migration function directly
		err = v2.MigrateStore(testCtx, kvStore, cdc)
		require.NoError(t, err)

		// Verify migration results - read params back from store
		paramsBz := kvStore.Get(types.ParamsKey)
		require.NotNil(t, paramsBz)

		var migratedParams types.Params
		err = cdc.Unmarshal(paramsBz, &migratedParams)
		require.NoError(t, err)

		// EpochInterval should be preserved from v1
		require.Equal(t, uint64(100), migratedParams.EpochInterval)

		// New fields should be set to defaults
		require.Equal(t, types.DefaultExecuteGas, migratedParams.ExecuteGas)
		require.Equal(t, types.DefaultMinAmount, migratedParams.MinAmount)

		// Verify params pass validation
		require.NoError(t, migratedParams.Validate())
	})

	// Test Case 2: Migration works correctly with existing v2 params (idempotency)
	t.Run("migration_success", func(t *testing.T) {
		// Perform migration (keepertest already sets default v2 params)
		m := keeper.NewMigrator(*epochingKeeper)
		require.NoError(t, m.Migrate1to2(ctx))

		// Verify migration completes successfully and params are valid
		params := epochingKeeper.GetParams(ctx)
		require.Equal(t, types.DefaultEpochInterval, params.EpochInterval)
		require.Equal(t, types.DefaultExecuteGas, params.ExecuteGas)
		require.Equal(t, types.DefaultMinAmount, params.MinAmount)

		// Verify params pass validation
		require.NoError(t, params.Validate())
	})
}
