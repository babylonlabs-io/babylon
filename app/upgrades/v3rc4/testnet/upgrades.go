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
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

const UpgradeName = "v3rc4"
const delegatePoolModuleName = "epoching_delegate_pool"

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
		if err := validateEpochBoundary(ctx, keepers.EpochingKeeper); err != nil {
			return nil, fmt.Errorf("epoch boundary validation failed: %w", err)
		}

		// Validate delegation pool module account exists before running migrations
		if err := validateDelegatePoolModuleAccount(ctx, keepers.AccountKeeper); err != nil {
			return nil, fmt.Errorf("spam prevention upgrade validation failed: %w", err)
		}
		// Validate that delegation pool has no locked funds
		if err := validateDelegatePoolEmpty(ctx, keepers.AccountKeeper, keepers.BankKeeper); err != nil {
			return nil, fmt.Errorf("delegate pool validation failed: %w", err)
		}

		// Run migrations (includes epoching v1->v2 migration)
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		if err := validateMigrationResults(ctx, keepers); err != nil {
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

// validateDelegatePoolModuleAccount validates that the delegation pool module account is properly configured
func validateDelegatePoolModuleAccount(ctx context.Context, ak types.AccountKeeper) error {
	// Use hardcoded module name to avoid dependency on upgraded types

	moduleAddr := ak.GetModuleAddress(delegatePoolModuleName)
	if moduleAddr == nil {
		return fmt.Errorf("module account %s has not been configured - ensure it's added to maccPerms in app.go",
			delegatePoolModuleName)
	}

	// Module account address exists, which means it's properly configured
	// The actual account object will be created when first used by the module
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("delegation pool module account validated",
		"module", delegatePoolModuleName,
		"address", moduleAddr.String())

	return nil
}

// validateDelegatePoolEmpty validates that the delegation pool module account has no locked funds
func validateDelegatePoolEmpty(ctx context.Context, ak types.AccountKeeper, bk types.BankKeeper) error {
	// Use hardcoded module name to avoid dependency on upgraded types
	moduleAddr := ak.GetModuleAddress(delegatePoolModuleName)

	balance := bk.SpendableCoins(ctx, moduleAddr)
	if !balance.IsZero() {
		return fmt.Errorf("upgrade cannot proceed with locked funds in delegation pool (balance: %s) - this indicates unprocessed queued messages with locked funds",
			balance.String())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("delegation pool validation successful",
		"module", delegatePoolModuleName,
		"address", moduleAddr.String(),
		"balance", balance.String())

	return nil
}

// validateEpochBoundary validates that upgrade happens at epoch boundary using epoching keeper
func validateEpochBoundary(ctx context.Context, epochingKeeper epochingkeeper.Keeper) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := uint64(sdkCtx.HeaderInfo().Height)

	// Get current epoch information
	currentEpoch := epochingKeeper.GetEpoch(ctx)

	// Handle special case: height 1 is always valid as first possible upgrade height
	if currentHeight == 1 {
		sdkCtx.Logger().Info("epoch boundary validation successful - height 1",
			"current_height", currentHeight,
			"note", "height 1 is always valid epoch boundary")
		return nil
	}

	if !currentEpoch.IsFirstBlockOfNextEpoch(ctx) {
		// Calculate next epoch boundary for error message using current epoch interval
		nextEpochHeight := currentEpoch.FirstBlockHeight + currentEpoch.CurrentEpochInterval

		return fmt.Errorf("upgrade must happen at epoch boundary - current height %d is not first block of next epoch (next epoch boundary at height %d, current epoch interval: %d)",
			currentHeight, nextEpochHeight, currentEpoch.CurrentEpochInterval)
	}

	sdkCtx.Logger().Info("epoch boundary validation successful",
		"current_height", currentHeight,
		"current_epoch", currentEpoch.EpochNumber,
		"epoch_interval", currentEpoch.CurrentEpochInterval,
		"is_epoch_boundary", true)

	return nil
}

func validateMigrationResults(ctx context.Context, keepers *keepers.AppKeepers) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate epoching params after migration
	epochingParams := keepers.EpochingKeeper.GetParams(sdkCtx)
	if err := epochingParams.Validate(); err != nil {
		return fmt.Errorf("migrated epoching params validation failed: %w", err)
	}

	sdkCtx.Logger().Info("migration validation successful",
		"epoch_interval", epochingParams.EpochInterval,
		"min_amount", epochingParams.MinAmount,
		"delegate_gas", epochingParams.ExecuteGas.Delegate)

	return nil
}
