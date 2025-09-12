package testnet

import (
	"context"
	"fmt"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/epoching"
)

const UpgradeName = "v3rc4"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		currentHeight := uint64(sdkCtx.HeaderInfo().Height)

		// Validate epoch boundary using epoching keeper
		if err := epoching.ValidateEpochBoundary(ctx, keepers.EpochingKeeper); err != nil {
			return nil, fmt.Errorf("epoch boundary validation failed: %w", err)
		}

		// Validate delegation pool module account exists before running migrations
		if err := epoching.ValidateDelegatePoolModuleAccount(ctx, keepers.AccountKeeper); err != nil {
			return nil, fmt.Errorf("spam prevention upgrade validation failed: %w", err)
		}
		// Validate that delegation pool has no locked funds
		if err := epoching.ValidateDelegatePoolEmpty(ctx, keepers.AccountKeeper, keepers.BankKeeper); err != nil {
			// Log warning instead of failing upgrade
			sdkCtx.Logger().Warn("delegate pool validation warning", "error", err.Error())
		}

		// Run migrations (includes epoching v1->v2 migration)
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		if err := epoching.ValidateMigrationResults(ctx, keepers); err != nil {
			return nil, fmt.Errorf("migration validation failed: %w", err)
		}
		// Log successful upgrade
		sdkCtx.Logger().Info("spam prevention upgrade completed successfully",
			"upgrade", UpgradeName,
			"epoching_migration", "v3rc3->v3rc4",
			"height", currentHeight,
			"epoch_boundary", true,
		)

		return migrations, nil
	}
}
