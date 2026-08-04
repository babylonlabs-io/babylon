package v4_4

import (
	"context"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
)

const UpgradeName = "v4.4"

// Upgrade carries no state migration. It exists to coordinate the binary swap
// for the cosmos-sdk v0.53.8 security release, which is state breaking: the SDK
// changes are unconditional binary behaviour, so every validator must switch at
// the same height or the network forks on the first affected transaction.
//
// No SDK module changed its ConsensusVersion between v0.53.4 (the version
// mainnet runs) and v0.53.8, so RunMigrations has nothing to migrate and no
// store keys are added or deleted.
var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, _ *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
