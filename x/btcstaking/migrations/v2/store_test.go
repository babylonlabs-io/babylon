package v2_test

import (
	"math/rand"
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"

	"github.com/stretchr/testify/require"
)

func TestMigrateStore(t *testing.T) {
	var (
		r                     = rand.New(rand.NewSource(time.Now().UnixNano()))
		storeKey              = storetypes.NewKVStoreKey(types.StoreKey)
		btcStakingKeeper, ctx = keepertest.BTCStakingKeeperWithStoreKey(t, storeKey, nil, nil, nil)
		paramsVersions        = 10
	)

	for i := 0; i < paramsVersions; i++ {
		params := types.DefaultParams()
		randomVersion := datagen.RandomInRange(r, 1, 100)
		params.MaxFinalityProviders = uint32(randomVersion)
		params.BtcActivationHeight = uint32(i + 1)
		require.NoError(t, btcStakingKeeper.SetParams(ctx, params))
	}

	// Perform migration
	m := keeper.NewMigrator(*btcStakingKeeper)
	require.NoError(t, m.Migrate1to2(ctx))

	// after migration, all params should have max finality providers set to 1
	for i := 0; i < paramsVersions; i++ {
		params := btcStakingKeeper.GetParamsByVersion(ctx, uint32(i))
		require.Equal(t, uint32(1), params.MaxFinalityProviders)
	}
}
