package keeper

import (
	v2 "github.com/babylonlabs-io/babylon/v4/x/btcstaking/migrations/v2"
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

// Migrate1to2 migrates from version 1 to 2.
// This migration adds support for multisig BTC stakers by ensuring
// MaxStakerQuorum and MaxStakerNum parameters are available.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	return v2.MigrateStore(ctx, m.k)
}