package v3

import (
	"context"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v3/app/keepers"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v3 upgrade
const UpgradeName = "v3"

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
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
