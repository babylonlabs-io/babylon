package keeper

import (
	v2 "github.com/babylonlabs-io/babylon/v3/x/epoching/migrations/v2"
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
// This migration adds ExecuteGas and MinAmount parameters to the existing Params.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	store := runtime.KVStoreAdapter(m.k.storeService.OpenKVStore(ctx))
	return v2.MigrateStore(ctx, store, m.k.cdc)
}
