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
	DeletedCapabilityStoreKey = "capability"
)

var StoresToAdd = []string{
	btcstkconsumertypes.StoreKey,
	zoneconciergetypes.StoreKey,
}

func CreateUpgrade(
	permissionedIntegration bool,
	fpCount, btcActivationHeight, ibcPacketTimeoutSeconds uint32,
) upgrades.Upgrade {
	return upgrades.Upgrade{
		UpgradeName: UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(
			permissionedIntegration,
			fpCount, btcActivationHeight, ibcPacketTimeoutSeconds,
		),
		StoreUpgrades: store.StoreUpgrades{
			Added: StoresToAdd,
			Deleted: []string{
				DeletedCapabilityStoreKey,
			},
		},
	}
}

func CreateUpgradeHandler(
	permissionedIntegration bool,
	fpCount, btcActivationHeight, ibcPacketTimeoutSeconds uint32,
) upgrades.UpgradeHandlerCreator {
	return func(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(goCtx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			migrations, err := mm.RunMigrations(goCtx, configurator, fromVM)
			if err != nil {
				return nil, err
			}

			ctx := sdk.UnwrapSDKContext(goCtx)

			zoneConciergeParams := zoneconciergetypes.DefaultParams()
			zoneConciergeParams.IbcPacketTimeoutSeconds = ibcPacketTimeoutSeconds
			if err = zoneConciergeParams.Validate(); err != nil {
				return nil, err
			}

			err = keepers.ZoneConciergeKeeper.SetParams(ctx, zoneConciergeParams)
			if err != nil {
				return nil, err
			}

			btcStkConsumerParams := btcstkconsumertypes.DefaultParams()
			btcStkConsumerParams.PermissionedIntegration = permissionedIntegration
			err = keepers.BTCStkConsumerKeeper.SetParams(ctx, btcStkConsumerParams)
			if err != nil {
				return nil, err
			}

			// from the last parameter version, updates the fp count and activation height
			btcParams := keepers.BTCStakingKeeper.GetParams(ctx)
			btcParams.MaxFinalityProviders = fpCount
			btcParams.BtcActivationHeight = btcActivationHeight

			err = keepers.BTCStakingKeeper.SetParams(ctx, btcParams)
			if err != nil {
				return nil, err
			}

			return migrations, nil
		}
	}
}
