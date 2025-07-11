package keeper

import (
	"context"

	v2 "github.com/babylonlabs-io/babylon/v3/x/btcstaking/migrations/v2"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	return v2.MigrateStore(ctx, store, m.keeper.cdc, func(ctx context.Context, p *types.Params) error {
		p.MaxFinalityProviders = 1
		return nil
	})
}
