package testnet_test

import (
	"math/rand"
	"sort"
	"testing"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades/v1rc8/testnet"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	btcstakingtypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/runtime"

	"github.com/test-go/testify/require"
)

func FuzzMigrateFinalityProviders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r            = rand.New(rand.NewSource(seed))
			storeKey     = storetypes.NewKVStoreKey(btcstakingtypes.StoreKey)
			storeService = runtime.NewKVStoreService(storeKey)
			encConf      = appparams.DefaultEncodingConfig()
			keeper, ctx  = keepertest.BTCStakingKeeperWithStoreKey(t, storeKey, nil, nil, nil)
		)

		// seed the store with finality providers without commission info
		storeAdapter := runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
		store := prefix.NewStore(storeAdapter, btcstakingtypes.FinalityProviderKey)
		bsnIndexStore := prefix.NewStore(storeAdapter, btcstakingtypes.FinalityProviderBsnIndexKey)
		fpCount := rand.Intn(300)
		// slice of the expected finality providers after the migration
		expFps := make([]btcstakingtypes.FinalityProvider, fpCount)
		for i := range expFps {
			fp, err := datagen.GenRandomFinalityProvider(r, "", "")
			require.NoError(t, err)
			// Set BSN ID to chain ID (since datagen creates empty BSN ID)
			fp.BsnId = ctx.ChainID()
			// make sure commission info is nil when seeding the store
			fp.CommissionInfo = nil
			// use store directly to store the fps
			fpBytes := encConf.Codec.MustMarshal(fp)
			store.Set(fp.BtcPk.MustMarshal(), fpBytes)

			// Create BSN index entry
			bsnKey := btcstakingtypes.BuildBsnIndexKey(fp.BsnId, fp.BtcPk)
			bsnIndexStore.Set(bsnKey, []byte{})

			// Add the expected fp with the commission info defined
			expFps[i] = btcstakingtypes.FinalityProvider{
				Addr:                 fp.Addr,
				Description:          fp.Description,
				Commission:           fp.Commission,
				BtcPk:                fp.BtcPk,
				Pop:                  fp.Pop,
				SlashedBabylonHeight: fp.SlashedBabylonHeight,
				SlashedBtcHeight:     fp.SlashedBtcHeight,
				Jailed:               fp.Jailed,
				HighestVotedHeight:   fp.HighestVotedHeight,
				BsnId:                fp.BsnId,
				CommissionInfo: btcstakingtypes.NewCommissionInfoWithTime(
					sdkmath.LegacyMustNewDecFromStr("0.2"),
					sdkmath.LegacyMustNewDecFromStr("0.01"),
					ctx.BlockHeader().Time,
				),
			}
		}

		// Run the migration logic
		require.NoError(t, testnet.MigrateFinalityProviders(ctx, *keeper))

		// get all the stored finality providers
		migratedFps := []btcstakingtypes.FinalityProvider{}
		iter := store.Iterator(nil, nil)
		defer iter.Close()
		for ; iter.Valid(); iter.Next() {
			var fp btcstakingtypes.FinalityProvider
			encConf.Codec.MustUnmarshal(iter.Value(), &fp)
			migratedFps = append(migratedFps, fp)
		}

		// sort the expected and migrated slices
		sort.Slice(expFps, func(i, j int) bool {
			return expFps[i].Addr < expFps[j].Addr
		})
		sort.Slice(migratedFps, func(i, j int) bool {
			return migratedFps[i].Addr < migratedFps[j].Addr
		})
		require.Equal(t, expFps, migratedFps)
	})
}
