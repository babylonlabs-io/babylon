package v4_7

import (
	"context"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
)

const UpgradeName = "v4.7"

// Upgrade coordinates the Go 1.26.10 and dependency security updates without
// custom migrations. Validation changes apply at process startup: validators
// must switch at the plan height, not before.
var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, _ *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
