package testnet

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const (
	UpgradeName = "v1rc5"
)

func CreateUpgrade() upgrades.Upgrade {
	return upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(),
	}
}

// CreateUpgradeHandler upgrade handler for launch.
func CreateUpgradeHandler() upgrades.UpgradeHandlerCreator {
	return func(mm *module.Manager, cfg module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(context context.Context, _plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			ctx := sdk.UnwrapSDKContext(context)

			migrations, err := mm.RunMigrations(ctx, cfg, fromVM)
			if err != nil {
				return nil, fmt.Errorf("failed to run migrations: %w", err)
			}

			return migrations, nil
		}
	}
}
