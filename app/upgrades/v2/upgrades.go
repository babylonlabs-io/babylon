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
	ratelimitkeeper "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/keeper"
	ratelimittypes "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	incentivekeeper "github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
)

const (
	// UpgradeName defines the on-chain upgrade name for the Babylon v2 upgrade
	UpgradeName = "v2"
	// InterchainQueryStoreName defines the hardcoded store name for the async-icq module,
	// as specified in the following commit:
	// https://github.com/cosmos/ibc-apps/blob/modules/async-icq/v8.0.0/modules/async-icq/types/keys.go#L14
	//
	// This store name is hardcoded for the following reasons:
	// - The async-icq module was introduced in v2rc0 and deployed on the testnet.
	// - Internal review showed limited usage and demand for the module.
	// - The Cosmos EVM package requires IBC v10, while async-icq depends on IBC v8,
	//   with no upgrade planned.
	//
	// As a result, we decided to remove the async-icq dependency.
	// However, to preserve a record of all upgrades applied to the testnet,
	// we retain the v2rc0 upgrade in our codebase and plan to remove async-icq
	// entirely in the subsequent v2rc2 upgrade.
	//
	// To fully decouple from the module now, we hardcode the store name here.
	InterchainQueryStoreName = "interchainquery"
	// CrisisStoreName `x/crisis` module is deprecated at cosmos-sdk v0.53 and
	// will be removed in the next release.
	CrisisStoreName = "crisis"
)

var (
	// durations in hours
	DailyDurationHours uint64 = 24
	// limits (percentages)
	DefaultDailyLimit = sdkmath.NewInt(10)
)

func CreateUpgrade(includeAsyncICQ bool, whitelistedChannelsByID map[string]struct{}) upgrades.Upgrade {
	addedStoreUpgrades := []string{tokenfactorytypes.StoreKey, pfmroutertypes.StoreKey, icacontrollertypes.StoreKey, icahosttypes.StoreKey, ratelimittypes.StoreKey}

	if includeAsyncICQ {
		addedStoreUpgrades = append(addedStoreUpgrades, InterchainQueryStoreName)
	}

	return upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(whitelistedChannelsByID),
		StoreUpgrades: store.StoreUpgrades{
			Added:   addedStoreUpgrades,
			Deleted: []string{ibcfeetypes.StoreKey, CrisisStoreName},
		},
	}
}

func CreateUpgradeHandler(whitelistedChannelsByID map[string]struct{}) upgrades.UpgradeHandlerCreator {
	return func(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			// Run migrations before applying any other state changes.
			migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
			if err != nil {
				return nil, err
			}

			sdkCtx := sdk.UnwrapSDKContext(ctx)

			// Add a default rate limit to existing channels on port 'transfer'
			if err := addRateLimits(sdkCtx, keepers.IBCKeeper.ChannelKeeper, keepers.RatelimitKeeper, whitelistedChannelsByID); err != nil {
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
}

// addRateLimits adds rate limit to the native denom ('ubbn') to all channels registered on the channel keeper
func addRateLimits(ctx sdk.Context, chk transfertypes.ChannelKeeper, rlk ratelimitkeeper.Keeper, whitelistedChannelsByID map[string]struct{}) error {
	logger := ctx.Logger().With("upgrade", UpgradeName)
	channels := chk.GetAllChannelsWithPortPrefix(ctx, transfertypes.PortID)
	logger.Info("adding limits to channels", "channels_count", len(channels))
	for _, ch := range channels {
		// if there is no whitelisted channel it sets to all the available transfer channels
		if len(whitelistedChannelsByID) != 0 {
			_, isWhitelisted := whitelistedChannelsByID[ch.ChannelId]
			if !isWhitelisted {
				continue
			}
		}

		logger.Info("adding limits to whitelist channel", "channel_id", ch.ChannelId)
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
