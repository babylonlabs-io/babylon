package v2

import (
	"context"

	sdkmath "cosmossdk.io/math"
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	store "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/v2/app/keepers"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	pfmroutertypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	ratelimittypes "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/types"
	ratelimitkeeper "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/keeper"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v2 upgrade
const (
	UpgradeName            = "v2"
	Denom                  = "ubbn"
	DefaultTransferChannel = "channel-5"
	NobleTransferChannel   = "channel-1"
	AtomTransferChannel    = "channel-0"

	// durations in hours
	DailyDurationHours  = 24
	WeeklyDurationHours = 168

	// limits (percentages)
	DefaultDailyLimit  = 20
	DefaultWeeklyLimit = 40

	NobleDailyLimit  = 30
	NobleWeeklyLimit = 60

	AtomDailyLimit  = 20
	AtomWeeklyLimit = 40
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{tokenfactorytypes.ModuleName, pfmroutertypes.StoreKey, ratelimittypes.StoreKey},
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

		sdkCtx := sdk.UnwrapSDKContext(ctx)

		if err := AddStackedRateLimit(sdkCtx, keepers.RatelimitKeeper, Denom, DefaultTransferChannel, []int64{DefaultDailyLimit, DefaultWeeklyLimit}, []int64{DailyDurationHours, WeeklyDurationHours}); err != nil {
			return nil, err
		}
		if err := AddStackedRateLimit(sdkCtx, keepers.RatelimitKeeper, Denom, NobleTransferChannel, []int64{NobleDailyLimit, NobleWeeklyLimit}, []int64{DailyDurationHours, WeeklyDurationHours}); err != nil {
			return nil, err
		}
		if err := AddStackedRateLimit(sdkCtx, keepers.RatelimitKeeper, Denom, AtomTransferChannel, []int64{AtomDailyLimit, AtomWeeklyLimit}, []int64{DailyDurationHours, WeeklyDurationHours}); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

func AddRateLimit(ctx sdk.Context, k ratelimitkeeper.Keeper, denom, channel string, percent int, durationHours uint64) error {
	addRateLimitMsg := ratelimittypes.MsgAddRateLimit{
		ChannelId:      channel,
		Denom:          denom,
		MaxPercentSend: sdkmath.NewInt(int64(percent)),
		MaxPercentRecv: sdkmath.NewInt(int64(percent)),
		DurationHours:  durationHours,
	}

	err := k.AddRateLimit(ctx, &addRateLimitMsg)
	if err != nil {
		panic(errorsmod.Wrapf(err, "unable to add rate limit for denom %s on channel %s", denom, channel))
	}
	return nil
}

func AddStackedRateLimit(ctx sdk.Context, k ratelimitkeeper.Keeper, denom, channel string, percents []int64, durations []int64) error {
	if len(percents) != len(durations) {
		return errorsmod.Wrapf(nil, "percents and durations must be the same length")
	}
	for i := range percents {
		if err := AddRateLimit(ctx, k, denom, channel, int(percents[i]), uint64(durations[i])); err != nil {
			return err
		}
	}
	return nil
}
