package v2_test

// import (
// 	"context"
// 	"math/rand"
// 	"testing"
// 	"time"

// 	"cosmossdk.io/core/store"
// 	"cosmossdk.io/store/prefix"
// 	storetypes "cosmossdk.io/store/types"
// 	"github.com/cosmos/cosmos-sdk/codec"
// 	"github.com/cosmos/cosmos-sdk/runtime"

// 	appparams "github.com/babylonlabs-io/babylon/app/params"
// 	"github.com/babylonlabs-io/babylon/testutil/datagen"
// 	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
// 	v2 "github.com/babylonlabs-io/babylon/x/btcstaking/migrations/v2"
// 	"github.com/babylonlabs-io/babylon/x/btcstaking/types"

// 	"github.com/test-go/testify/require"
// )

// func TestMigrateStore(t *testing.T) {
// 	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
// 	storeService := runtime.NewKVStoreService(storeKey)
// 	encCfg := appparams.DefaultEncodingConfig()
// 	k, ctx := testkeeper.BTCStakingKeeperWithStoreKey(t, storeKey, nil, nil, nil)

// 	// setup some finality providers
// 	expFp := setupFinalityProviers(t, ctx, storeService, encCfg.Codec)

// 	// Run migrations
// 	store := runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
// 	v2.MigrateStore(ctx, store, k, encCfg.Codec)

// 	// validate finality providers
// 	for _, fp := range expFp {
// 		updatedFp, err := k.GetFinalityProvider(ctx, *fp.BtcPk)
// 		require.NoError(t, err)
// 		require.Equal(t, fp, updatedFp)
// 		require.Equal(t, time.Unix(0, 0).UTC(), updatedFp.CommissionInfo.UpdateTime)
// 	}
// }

// // setupFinalityProviers sets up some finality providers for the test and
// // returns the expected finality providers after the migration
// func setupFinalityProviers(t *testing.T, ctx context.Context, storeService store.KVStoreService, cdc codec.Codec) []*types.FinalityProvider {
// 	r := rand.New(rand.NewSource(time.Now().Unix()))
// 	fpCount := rand.Intn(20)
// 	exp := make([]*types.FinalityProvider, fpCount)
// 	storeAdapter := runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
// 	fpStore := prefix.NewStore(storeAdapter, types.FinalityProviderKey)
// 	for i := range exp {
// 		var err error
// 		exp[i], err = datagen.GenRandomFinalityProvider(r)
// 		require.NoError(t, err)
// 		// create fp without CommissionUpdateTime
// 		initFp := types.FinalityProvider{
// 			Description: exp[i].Description,
// 			Commission:  exp[i].Commission,
// 			Addr:        exp[i].Addr,
// 			BtcPk:       exp[i].BtcPk,
// 			Pop:         exp[i].Pop,
// 			ConsumerId:  exp[i].ConsumerId,
// 		}
// 		fpBz := cdc.MustMarshal(&initFp)
// 		fpStore.Set(initFp.BtcPk.MustMarshal(), fpBz)
// 	}
// 	return exp
// }
