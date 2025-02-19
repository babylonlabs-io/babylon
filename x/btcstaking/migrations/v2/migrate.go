package v2

import (
	"time"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// MigrateStore performs store migrations to add the new fields
// needed for validation on finality provider commission updates.
// The migration includes:
// - Adding the commission max change rate to the module parameters
// - Adding the field CommissionUpdateTime to the FinalityProvier type
func MigrateStore(store storetypes.KVStore, cdc codec.BinaryCodec) error {
	// Migrate params - add the max change rate default value
	if err := migrateParams(store, cdc); err != nil {
		return err
	}

	// Migrate FP to have a default updated timestamp
	migrateFinalityProviders(store, cdc)

	return nil
}

// migrateParams adds the default value to the new param max commission change rate
func migrateParams(store storetypes.KVStore, cdc codec.BinaryCodec) error {
	var params types.Params
	defaultParams := types.DefaultParams()
	paramsBz := store.Get(types.ParamsKey)
	cdc.MustUnmarshal(paramsBz, &params)

	params.MaxCommissionChangeRate = defaultParams.MaxCommissionChangeRate
	if err := params.Validate(); err != nil {
		return err
	}

	bz := cdc.MustMarshal(&params)

	store.Set(types.ParamsKey, bz)
	return nil
}

// migrateFinalityProviders adds a default value to the new CommissionUpdateTime
// field on the FinalityProviders
func migrateFinalityProviders(store storetypes.KVStore, cdc codec.BinaryCodec) {
	fpStore := prefix.NewStore(store, types.FinalityProviderKey)
	iter := fpStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var fp types.FinalityProvider
		cdc.MustUnmarshal(iter.Value(), &fp)

		// Add the commission update time to the finality providers
		fp.CommissionUpdateTime = time.Unix(0, 0).UTC()

		updatedFpBz := cdc.MustMarshal(&fp)
		fpStore.Set(iter.Key(), updatedFpBz)
	}
}
