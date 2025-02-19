package v2

import (
	"context"
	"time"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

type Keeper interface {
	OverwriteParamsAtVersion(ctx context.Context, v uint32, p types.Params) error
	GetParamsWithVersion(ctx context.Context) types.StoredParams
}

// MigrateStore performs store migrations to add the new fields
// needed for validation on finality provider commission updates.
// The migration includes:
// - Adding the commission max change rate to the module parameters
// - Adding the field CommissionUpdateTime to the FinalityProvier type
func MigrateStore(ctx context.Context, store storetypes.KVStore, k Keeper, cdc codec.BinaryCodec) error {
	// Migrate params - add the max change rate default value
	if err := migrateParams(ctx, k); err != nil {
		return err
	}

	// Migrate FP to have a default updated timestamp
	migrateFinalityProviders(store, cdc)

	return nil
}

// migrateParams adds the default value to the new param max commission change rate
func migrateParams(ctx context.Context, k Keeper) error {
	storedParams := k.GetParamsWithVersion(ctx)
	storedParams.Params.MaxCommissionChangeRate = types.DefaultMaxCommissionChangeRate
	if err := storedParams.Params.Validate(); err != nil {
		return err
	}
	return k.OverwriteParamsAtVersion(ctx, storedParams.Version, storedParams.Params)
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
