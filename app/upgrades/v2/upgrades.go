package v2

import (
	"context"
	store "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/v2/app/keepers"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	"github.com/babylonlabs-io/babylon/v2/x/mint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	pfmroutertypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v2 upgrade
const UpgradeName = "v2"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{tokenfactorytypes.ModuleName, pfmroutertypes.StoreKey},
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// Run migrations before applying any other state changes.
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		// Set the denom creation fee to ubbn
		params := tokenfactorytypes.DefaultParams()
		params.DenomCreationFee = sdk.NewCoins(sdk.NewInt64Coin(types.DefaultBondDenom, 10_000_000))

		if err := params.Validate(); err != nil {
			return nil, err
		}

		if err := keepers.TokenFactoryKeeper.SetParams(ctx, params); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}
