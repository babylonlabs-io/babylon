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
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types/allowlist"

	"github.com/stretchr/testify/require"
)

func TestMigrateStore(t *testing.T) {
	var (
		r                     = rand.New(rand.NewSource(time.Now().UnixNano()))
		storeKey              = storetypes.NewKVStoreKey(types.StoreKey)
		btcStakingKeeper, ctx = keepertest.BTCStakingKeeperWithStoreKey(t, storeKey, nil, nil, nil, nil)
		paramsVersions        = 10
		testChainID           = "test-chain-id"
		nFps                  = 10
	)

	for i := 0; i < paramsVersions; i++ {
		params := types.DefaultParams()
		randomVersion := datagen.RandomInRange(r, 1, 100)
		params.MaxFinalityProviders = uint32(randomVersion)
		params.BtcActivationHeight = uint32(i + 1)
		require.NoError(t, btcStakingKeeper.SetParams(ctx, params))
	}

	var generatedRandomFps []*types.FinalityProvider
	for i := 0; i < nFps; i++ {
		fp, err := datagen.GenRandomFinalityProvider(r, "", "")
		require.NoError(t, err)
		btcStakingKeeper.SetFinalityProvider(ctx, fp)
		generatedRandomFps = append(generatedRandomFps, fp)
	}

	// Perform migration
	m := keeper.NewMigrator(*btcStakingKeeper)
	require.NoError(t, m.Migrate1to2(ctx.WithChainID(testChainID)))

	// after migration, all params should have max finality providers set to 1
	for i := 0; i < paramsVersions; i++ {
		params := btcStakingKeeper.GetParamsByVersion(ctx, uint32(i))
		require.Equal(t, uint32(1), params.MaxFinalityProviders)
	}

	// check if multi-staking allow list is indexed
	txHashes, err := allowlist.LoadMultiStakingAllowList()
	require.NoError(t, err)
	require.NotEmpty(t, txHashes)
	store := ctx.KVStore(storeKey)
	for _, txHash := range txHashes {
		key := append(types.AllowedMultiStakingTxHashesKey, txHash[:]...) //nolint:gocritic
		exists := store.Has(key)
		require.True(t, exists, "tx hash %s should be indexed in the allow list", txHash.String())
	}

	// check if all finality providers have the BSN ID set to the test chain ID
	for _, fp := range generatedRandomFps {
		fp, err := btcStakingKeeper.GetFinalityProvider(ctx, fp.BtcPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, testChainID, fp.BsnId)
	}

	// check if all finality providers are properly indexed
	fpResp, err := btcStakingKeeper.FinalityProviders(ctx, &types.QueryFinalityProvidersRequest{
		BsnId: testChainID,
	})
	require.NoError(t, err)
	require.Equal(t, len(generatedRandomFps), len(fpResp.FinalityProviders))
	for _, fp := range fpResp.FinalityProviders {
		require.Equal(t, testChainID, fp.BsnId)
	}
}
