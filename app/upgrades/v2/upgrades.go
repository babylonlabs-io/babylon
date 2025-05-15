package v2

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	pfmroutertypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	icqtypes "github.com/cosmos/ibc-apps/modules/async-icq/v8/types"
	ratelimitkeeper "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/keeper"
	ratelimittypes "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	"github.com/babylonlabs-io/babylon/v2/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/v2/app/params"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	incentivekeeper "github.com/babylonlabs-io/babylon/v2/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/v2/x/mint/types"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v2 upgrade
const UpgradeName = "v2"

var (
	// durations in hours
	DailyDurationHours uint64 = 24
	// limits (percentages)
	DefaultDailyLimit = sdkmath.NewInt(20)

	Upgrade = upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler,
		StoreUpgrades: store.StoreUpgrades{
			Added:   []string{tokenfactorytypes.ModuleName, pfmroutertypes.StoreKey, icacontrollertypes.StoreKey, icahosttypes.StoreKey, icqtypes.StoreKey, ratelimittypes.StoreKey},
			Deleted: []string{ibcfeetypes.StoreKey},
		},
	}
)

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// Run migrations before applying any other state changes.
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)

		// Add a default rate limit to existing channels on port 'transfer'
		if err := addRateLimits(sdkCtx, keepers.IBCKeeper.ChannelKeeper, keepers.RatelimitKeeper); err != nil {
			return nil, err
		}

		// By default, ICQ allowed queries are empty. So no queries will be allowed until
		// the allowed list is populated via gov proposal.
		// For ICA host, by default all messages are allowed (using '*' wildcard),
		// so we set allow list to empty and messages can be added later when needed via gov proposal
		icaHostParams := icahosttypes.DefaultParams()
		icaHostParams.AllowMessages = nil
		if err := icaHostParams.Validate(); err != nil {
			return nil, err
		}
		keepers.ICAHostKeeper.SetParams(sdkCtx, icaHostParams)

		// update reward distribution events
		err = UpdateRewardTrackerEventLastProcessedHeight(ctx, keepers.IncentiveKeeper)
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

// addRateLimits adds rate limit to the native denom ('ubbn') to all channels registered on the channel keeper
func addRateLimits(ctx sdk.Context, chk transfertypes.ChannelKeeper, rlk ratelimitkeeper.Keeper) error {
	logger := ctx.Logger().With("upgrade", UpgradeName)
	channels := chk.GetAllChannelsWithPortPrefix(ctx, transfertypes.PortID)
	logger.Info("adding limits to channels", "channels_count", len(channels))
	for _, ch := range channels {
		if err := addRateLimit(ctx, rlk, appparams.DefaultBondDenom, ch.ChannelId, DefaultDailyLimit, DailyDurationHours); err != nil {
			return err
		}
	}
	logger.Info("Done adding limits to channels")
	return nil
}

func addRateLimit(ctx sdk.Context, k ratelimitkeeper.Keeper, denom, channel string, percent sdkmath.Int, durationHours uint64) error {
	addRateLimitMsg := ratelimittypes.MsgAddRateLimit{
		ChannelId:      channel,
		Denom:          denom,
		MaxPercentSend: percent,
		MaxPercentRecv: percent,
		DurationHours:  durationHours,
	}

	err := k.AddRateLimit(ctx, &addRateLimitMsg)
	if err != nil {
		panic(errorsmod.Wrapf(err, "unable to add rate limit for denom %s on channel %s", denom, channel))
	}
	return nil
}

// UpdateRewardTrackerEventLastProcessedHeight sets the current block height - 1 to the reward tracker
// so that BTC reward distribution starts from the height at which the upgrade happens
func UpdateRewardTrackerEventLastProcessedHeight(goCtx context.Context, ictvK incentivekeeper.Keeper) error {
	ctx := sdk.UnwrapSDKContext(goCtx)
	blkHeight := uint64(ctx.HeaderInfo().Height) - 1 // previous block as it can have events processed at the upgrade height
	return ictvK.SetRewardTrackerEventLastProcessedHeight(ctx, blkHeight)
}
