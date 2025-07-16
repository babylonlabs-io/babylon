package v3

import (
	"context"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/babylonlabs-io/babylon/v3/app/keepers"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	"github.com/cosmos/cosmos-sdk/types/module"

	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	zoneconciergetypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v3 upgrade
const UpgradeName = "v3"

const deletedCapabilityStoreKey = "capability"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added: []string{
			btcstkconsumertypes.StoreKey,
			zoneconciergetypes.StoreKey,
		},
		Deleted: []string{
			deletedCapabilityStoreKey,
		},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		newVM, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return newVM, err
		}

		// ensure that the parameter is 1 when during the validation
		params := keepers.BTCStakingKeeper.GetParams(ctx)
		params.MaxFinalityProviders = 1

		if err := keepers.BTCStakingKeeper.SetParams(ctx, params); err != nil {
			return newVM, err
		}

		return newVM, nil
	}
}
