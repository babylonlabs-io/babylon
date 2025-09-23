package keeper

import (
	v3 "github.com/babylonlabs-io/babylon/v4/x/incentive/migrations/v3"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	k Keeper
}

// NewMigrator returns a new Migrator instance.
func NewMigrator(k Keeper) Migrator {
	return Migrator{
		k: k,
	}
}

// Migrate2to3 migrates from version 2 to 3.
func (m Migrator) Migrate2to3(ctx sdk.Context) error {
	return v3.MigrateStore(ctx, m.k)
}
