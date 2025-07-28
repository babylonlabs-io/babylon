package v3

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/babylonlabs-io/babylon/v3/app/keepers"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	"github.com/cosmos/cosmos-sdk/types/module"

	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	zoneconciergetypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v3 upgrade
const (
	UpgradeName               = "v3"
	deletedCapabilityStoreKey = "capability"
)

func CreateUpgrade(isMainnet bool) upgrades.Upgrade {
	return upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(isMainnet),
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
}

func CreateUpgradeHandler(isMainnet bool) upgrades.UpgradeHandlerCreator {
	return func(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
			if err != nil {
				return nil, err
			}

			sdkCtx := sdk.UnwrapSDKContext(ctx)

			btcParams := keepers.BTCStakingKeeper.GetParams(sdkCtx)
			btcParamsCopy := &btcParams
			var heightIncrement uint32
			if isMainnet {
				btcParamsCopy.MaxFinalityProviders = 5
				heightIncrement = 288
			} else {
				btcParamsCopy.MaxFinalityProviders = 10
				heightIncrement = 144
			}
			btcParamsCopy.BtcActivationHeight = btcParamsCopy.
				BtcActivationHeight + heightIncrement
			err = keepers.BTCStakingKeeper.SetParams(sdkCtx, *btcParamsCopy)
			return migrations, nil
		}
	}
}
