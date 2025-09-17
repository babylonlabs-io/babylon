package testnet

import (
	"context"
	"errors"
	"fmt"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/epoching"
	v3rc4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc4"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

const UpgradeName = "v3rc4"

var StoresToAdd = []string{
	costktypes.StoreKey,
	// evm
	erc20types.StoreKey, evmtypes.StoreKey, feemarkettypes.StoreKey, precisebanktypes.StoreKey,
}

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   StoresToAdd,
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
			sdkCtx.Logger().Warn("delegate pool had non-zero balance but failed to transfer funds to fee collector - upgrade proceeding", "error", err.Error())
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

		// coostaking upgrade
		costkStoreKey := keepers.GetKey(costktypes.StoreKey)
		if costkStoreKey == nil {
			return nil, errors.New("invalid costaking types store key")
		}

		coStkStoreService := runtime.NewKVStoreService(costkStoreKey)
		if err := v3rc4.InitializeCoStakerRwdsTracker(ctx, keepers.EncCfg.Codec, coStkStoreService, keepers.StakingKeeper, keepers.BTCStakingKeeper, keepers.CostakingKeeper, keepers.FinalityKeeper); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}
