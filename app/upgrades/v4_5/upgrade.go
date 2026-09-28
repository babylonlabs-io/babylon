package v4_5

import (
	"context"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
)

const UpgradeName = "v4.5"

// Upgrade carries no state migration. It exists to coordinate the binary swap
// for the CosmWasm wasmd v0.60.9 / wasmvm v2.3.5 security hotfix, which is
// state breaking: the fix changes how x/wasm handles contract results and
// account cleanup, so every validator must switch at the same height or the
// network forks on the first affected transaction.
//
// x/wasm's ConsensusVersion is unchanged between v0.60.5 (the version v4.4
// runs) and v0.60.9, so RunMigrations has nothing to migrate and no store keys
// are added or deleted.
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
