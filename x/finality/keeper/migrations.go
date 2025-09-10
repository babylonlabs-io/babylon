package keeper

import (
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	v2 "github.com/babylonlabs-io/babylon/v4/x/finality/migrations/v2"
	v3 "github.com/babylonlabs-io/babylon/v4/x/finality/migrations/v3"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator instance.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{
		keeper: keeper,
	}
}

// Migrate1to2 migrates from version 1 to 2.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	store := runtime.KVStoreAdapter(m.keeper.storeService.OpenKVStore(ctx))
	return v2.MigrateStore(ctx, store, m.keeper.upsertPubRandCommitIdx)
}

// Migrate2to3 migrates from version 2 to 3.
func (m Migrator) Migrate2to3(ctx sdk.Context) error {
	return v3.MigrateStore(ctx, m.keeper)
}
