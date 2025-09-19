package v5

import (
	"context"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v5 upgrade
const UpgradeName = "v5"

func clearAllHistoricalEpochMsgs(ctx sdk.Context, k epochingkeeper.Keeper) {
	currentEpoch := k.GetEpoch(ctx).EpochNumber

	// Clean all epoch messages before the current epoch
	for epochNum := uint64(1); epochNum < currentEpoch; epochNum++ {
		if k.GetQueueLength(ctx, epochNum) > 0 {
			k.ClearEpochMsgs(ctx, epochNum)
		}
	}
}

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(goCtx)

		// Clean all epoch messages during initialization
		clearAllHistoricalEpochMsgs(ctx, keepers.EpochingKeeper)

		return mm.RunMigrations(goCtx, configurator, fromVM)
	}
}
