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

func CreateUpgrade(fpCount uint32, btcActivationHeight uint32,
	permissionedIntegration bool, ibcPacketTimeoutSeconds uint32,
) upgrades.Upgrade {
	return upgrades.Upgrade{
		UpgradeName: UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(fpCount,
			btcActivationHeight, permissionedIntegration, ibcPacketTimeoutSeconds),
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

func CreateUpgradeHandler(fpCount uint32, btcActivationHeight uint32,
	permissionedIntegration bool, ibcPacketTimeoutSeconds uint32) upgrades.
UpgradeHandlerCreator {
	return func(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
			if err != nil {
				return nil, err
			}

			sdkCtx := sdk.UnwrapSDKContext(ctx)

			zoneConciergeParams := zoneconciergetypes.DefaultParams()
			zoneConciergeParams.
				IbcPacketTimeoutSeconds = ibcPacketTimeoutSeconds
			if err = zoneConciergeParams.Validate(); err != nil {
				return nil, err
			}

			err = keepers.ZoneConciergeKeeper.SetParams(sdkCtx,
				zoneConciergeParams)
			if err != nil {
				return nil, err
			}

			btcStkConsumerParams := btcstkconsumertypes.DefaultParams()
			btcStkConsumerParams.PermissionedIntegration = permissionedIntegration
			if err = btcStkConsumerParams.Validate(); err != nil {
				return nil, err
			}

			err = keepers.BTCStkConsumerKeeper.SetParams(sdkCtx,
				btcStkConsumerParams)
			if err != nil {
				return nil, err
			}

			btcParams := keepers.BTCStakingKeeper.GetParams(sdkCtx)
			btcParamsCopy := btcParams

			btcParamsCopy.MaxFinalityProviders = fpCount
			btcParamsCopy.BtcActivationHeight = btcActivationHeight

			err = keepers.BTCStakingKeeper.SetParams(sdkCtx, btcParamsCopy)
			if err != nil {
				return nil, err
			}

			return migrations, nil
		}
	}
}
