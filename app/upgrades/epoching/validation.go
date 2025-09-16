package epoching

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// ValidateDelegatePoolModuleAccount validates that the delegation pool module account is properly configured
func ValidateDelegatePoolModuleAccount(ctx context.Context, ak types.AccountKeeper) error {
	moduleAddr := ak.GetModuleAddress(types.DelegatePoolModuleName)
	if moduleAddr == nil {
		return fmt.Errorf("module account %s has not been configured - ensure it's added to maccPerms in app.go",
			types.DelegatePoolModuleName)
	}

	// Module account address exists, which means it's properly configured
	// The actual account object will be created when first used by the module
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("delegation pool module account validated",
		"module", types.DelegatePoolModuleName,
		"address", moduleAddr.String())

	return nil
}

// ValidateDelegatePoolEmpty validates that the delegation pool module account has no locked funds
func ValidateDelegatePoolEmpty(ctx context.Context, ak types.AccountKeeper, bk types.BankKeeper) error {
	moduleAddr := ak.GetModuleAddress(types.DelegatePoolModuleName)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	balance := bk.GetAllBalances(ctx, moduleAddr)
	if !balance.IsZero() {
		// Try to send to fee collector
		if err := bk.SendCoinsFromModuleToModule(ctx, types.DelegatePoolModuleName, authtypes.FeeCollectorName, balance); err != nil {
			sdkCtx.Logger().Warn("failed to transfer delegate pool funds to fee collector",
				"error", err.Error(), "balance", balance.String())
			return fmt.Errorf("delegation pool has locked funds that couldn't be transferred (balance: %s): %w",
				balance.String(), err)
		}
		sdkCtx.Logger().Info("transferred delegate pool funds to fee collector during upgrade",
			"amount", balance.String(),
			"from_module", types.DelegatePoolModuleName,
			"to_module", authtypes.FeeCollectorName)
	}

	sdkCtx.Logger().Info("delegation pool validation successful",
		"module", types.DelegatePoolModuleName,
		"address", moduleAddr.String(),
		"balance", balance.String())

	return nil
}

// ValidateEpochBoundary validates that upgrade happens at epoch boundary using epoching keeper
func ValidateEpochBoundary(ctx context.Context, epochingKeeper epochingkeeper.Keeper) error {
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

// ValidateMigrationResults validates the results of the migration using epoching keeper
func ValidateMigrationResults(ctx context.Context, keepers *keepers.AppKeepers) error {
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
