package v2_test

import (
	"context"
	"encoding/binary"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/store"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	v2 "github.com/babylonlabs-io/babylon/x/btcstaking/migrations/v2"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"

	"github.com/test-go/testify/require"
)

func TestMigrateStore(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	encCfg := appparams.DefaultEncodingConfig()
	k, ctx := testkeeper.BTCStakingKeeperWithStoreKey(t, storeKey, nil, nil, nil)

	// setup initial params without MaxCommissionChangeRate
	defaultParams := setupParams(t, ctx, storeService, k, encCfg.Codec)
	// when getting the params, because MaxCommissionChangeRate is nil, will default to 0 legacy dec
	initParams := k.GetParams(ctx)
	require.Equal(t, sdkmath.LegacyZeroDec(), initParams.MaxCommissionChangeRate)

	// setup some finality providers
	expFp := setupFinalityProviers(t, ctx, storeService, encCfg.Codec)

	// Run migrations
	store := runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
	v2.MigrateStore(ctx, store, k, encCfg.Codec)

	// Validate params
	updatedParams := k.GetParams(ctx)
	require.Equal(t, defaultParams, updatedParams)
	require.Equal(t, sdkmath.LegacyNewDecWithPrec(1, 1), updatedParams.MaxCommissionChangeRate)

	// validate finality providers
	for _, fp := range expFp {
		updatedFp, err := k.GetFinalityProvider(ctx, *fp.BtcPk)
		require.NoError(t, err)
		require.Equal(t, fp, updatedFp)
		require.Equal(t, time.Unix(0, 0).UTC(), updatedFp.CommissionUpdateTime)
	}
}

func uint32ToBytes(v uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	return buf[:]
}

// setupParams sets up initial params without MaxCommissionChangeRate.
// Returns the default params that are the expected params after the migration
func setupParams(t *testing.T, ctx context.Context, storeService store.KVStoreService, k *keeper.Keeper, cdc codec.Codec) types.Params {
	defaultParams := types.DefaultParams()
	initialParams := types.Params{
		CovenantPks:                  defaultParams.CovenantPks,
		CovenantQuorum:               defaultParams.CovenantQuorum,
		MinStakingValueSat:           defaultParams.MinStakingValueSat,
		MaxStakingValueSat:           defaultParams.MaxStakingValueSat,
		MinStakingTimeBlocks:         defaultParams.MinStakingTimeBlocks,
		MaxStakingTimeBlocks:         defaultParams.MaxStakingTimeBlocks,
		SlashingPkScript:             defaultParams.SlashingPkScript,
		MinSlashingTxFeeSat:          defaultParams.MinSlashingTxFeeSat,
		MinCommissionRate:            defaultParams.MinCommissionRate,
		SlashingRate:                 defaultParams.SlashingRate,
		UnbondingTimeBlocks:          defaultParams.UnbondingTimeBlocks,
		UnbondingFeeSat:              defaultParams.UnbondingFeeSat,
		DelegationCreationBaseGasFee: defaultParams.DelegationCreationBaseGasFee,
		AllowListExpirationHeight:    defaultParams.AllowListExpirationHeight,
		BtcActivationHeight:          defaultParams.BtcActivationHeight,
	}
	heightToVersionMap := k.GetHeightToVersionMap(ctx)
	require.Len(t, heightToVersionMap.Pairs, 1)
	storeAdapter := runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
	paramsStore := prefix.NewStore(storeAdapter, types.ParamsKey)
	initSp := types.StoredParams{
		Params:  initialParams,
		Version: 0,
	}
	// replace the existing params (default ones) with the initial params
	// without the MaxCommissionChangeRate field
	paramsStore.Set(uint32ToBytes(0), cdc.MustMarshal(&initSp))
	require.NoError(t, k.SetHeightToVersionMap(ctx, heightToVersionMap))
	return defaultParams
}

// setupFinalityProviers sets up some finality providers for the test and
// returns the expected finality providers after the migration
func setupFinalityProviers(t *testing.T, ctx context.Context, storeService store.KVStoreService, cdc codec.Codec) []*types.FinalityProvider {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	fpCount := rand.Intn(20)
	exp := make([]*types.FinalityProvider, fpCount)
	storeAdapter := runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
	fpStore := prefix.NewStore(storeAdapter, types.FinalityProviderKey)
	for i := range exp {
		var err error
		exp[i], err = datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		// create fp without CommissionUpdateTime
		initFp := types.FinalityProvider{
			Description: exp[i].Description,
			Commission:  exp[i].Commission,
			Addr:        exp[i].Addr,
			BtcPk:       exp[i].BtcPk,
			Pop:         exp[i].Pop,
			ConsumerId:  exp[i].ConsumerId,
		}
		fpBz := cdc.MustMarshal(&initFp)
		fpStore.Set(initFp.BtcPk.MustMarshal(), fpBz)
	}
	return exp
}
