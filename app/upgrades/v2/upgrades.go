package v2

import (
	"context"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	pfmroutertypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	icqtypes "github.com/cosmos/ibc-apps/modules/async-icq/v8/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	"github.com/babylonlabs-io/babylon/v2/app/keepers"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	incentivekeeper "github.com/babylonlabs-io/babylon/v2/x/incentive/keeper"
	minttypes "github.com/babylonlabs-io/babylon/v2/x/mint/types"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v2 upgrade
const UpgradeName = "v2"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{tokenfactorytypes.ModuleName, pfmroutertypes.StoreKey, icacontrollertypes.StoreKey, icahosttypes.StoreKey, icqtypes.StoreKey},
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

		// By default, ICQ allowed queries are empty. So no queries will be allowed until
		// the allowed list is populated via gov proposal.
		// For ICA host, by default all messages are allowed (using '*' wildcard),
		// so we set allow list to empty on so messages are added later when needed via gov proposal
		icaHostParams := icahosttypes.DefaultParams()
		icaHostParams.AllowMessages = nil
		if err := icaHostParams.Validate(); err != nil {
			return nil, err
		}
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		keepers.ICAHostKeeper.SetParams(sdkCtx, icaHostParams)

		// update reward distribution events
		err = UpdateRewardTrackerEventLastProcessedHeight(ctx, keepers.IncentiveKeeper)
		if err != nil {
			return nil, err
		}

		// Set the denom creation fee to ubbn
		params := tokenfactorytypes.DefaultParams()
		params.DenomCreationFee = sdk.NewCoins(sdk.NewInt64Coin(minttypes.DefaultBondDenom, 10_000_000))

		if err := params.Validate(); err != nil {
			return nil, err
		}

		if err := keepers.TokenFactoryKeeper.SetParams(ctx, params); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

// UpdateRewardTrackerEventLastProcessedHeight sets the current block height - 1 to the reward tracker
// so that BTC reward distribution starts from the height at which the upgrade happens
func UpdateRewardTrackerEventLastProcessedHeight(goCtx context.Context, ictvK incentivekeeper.Keeper) error {
	ctx := sdk.UnwrapSDKContext(goCtx)
	blkHeight := uint64(ctx.HeaderInfo().Height) - 1 // previous block as it can have events processed at the upgrade height
	return ictvK.SetRewardTrackerEventLastProcessedHeight(ctx, blkHeight)
}
