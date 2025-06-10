package testnet

import (
	"context"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v2/app/keepers"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	v2 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v2"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const UpgradeName = "v2rc4"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{v2.InterchainQueryStoreName, v2.CrisisStoreName},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// Run migrations before applying any other state changes.
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		return migrations, nil
	}
}
