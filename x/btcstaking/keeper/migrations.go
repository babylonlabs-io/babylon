package keeper

import (
	"context"

	v2 "github.com/babylonlabs-io/babylon/v4/x/btcstaking/migrations/v2"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	k Keeper
}

// NewMigrator returns a new Migrator instance.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{
		k: keeper,
	}
}

// Migrate1to2 migrates from version 1 to 2.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	store := runtime.KVStoreAdapter(m.k.storeService.OpenKVStore(ctx))
	return v2.MigrateStore(
		ctx,
		store,
		m.k.cdc,
		func(ctx context.Context, p *types.Params) error {
			p.MaxFinalityProviders = 1
			return nil
		},
		m.k.IndexAllowedMultiStakingTransaction,
		m.k.migrateBabylonFinalityProviders,
	)
}

// migrateBabylonFinalityProviders migrates all existing Babylon finality providers
// to to have the BSN ID set to Babylon's chain ID. It also indexes the finality
// provider in the BSN index store.
func (k Keeper) migrateBabylonFinalityProviders(ctx sdk.Context) {
	babylonBSNID := ctx.ChainID()

	store := k.finalityProviderStore(ctx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var fp types.FinalityProvider
		k.cdc.MustUnmarshal(iter.Value(), &fp)
		fp.BsnId = babylonBSNID
		k.SetFinalityProvider(ctx, &fp)
		k.bsnIndexFinalityProvider(ctx, &fp)
	}
}
