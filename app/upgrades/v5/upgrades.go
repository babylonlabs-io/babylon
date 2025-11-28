package v5

import (
	"context"
	"fmt"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

const UpgradeName = "v5"

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

		// run migrations (includes btcstaking v1->v2 migration for multisig support)
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		// log successful upgrade
		btcStakingPrevVersion := fromVM[bstypes.ModuleName]
		btcStakingNewVersion := migrations[bstypes.ModuleName]
		sdkCtx.Logger().Info("multisig BTC staker upgrade completed successfully",
			"upgrade", UpgradeName,
			"btcstaking_migration", fmt.Sprintf("v%d->v%d", btcStakingPrevVersion, btcStakingNewVersion),
			"height", currentHeight,
			"epoch_boundary", true,
		)

		return migrations, nil
	}
}
