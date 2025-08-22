package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService corestoretypes.KVStoreService

		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string
		// name of the FeeCollector ModuleAccount
		feeCollectorName string

		bankK types.BankKeeper
		accK  types.AccountKeeper

		// params stores the module parameter
		params collections.Item[types.Params]

		// Collections structures for rewards

		// currentRewards stores the current rewards information
		currentRewards collections.Item[types.CurrentRewards]
		// historicalRewards maps (period) => historicalRewards
		historicalRewards collections.Map[uint64, types.HistoricalRewards]
		// coostakerRewardsTracker maps (coostakerAddr) => coostakerRewardsTracker
		coostakerRewardsTracker collections.Map[[]byte, types.CoostakerRewardsTracker]
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	bankK types.BankKeeper,
	accK types.AccountKeeper,
	authority string,
	feeCollectorName string,
) Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	return Keeper{
		cdc:          cdc,
		storeService: storeService,

		authority:        authority,
		feeCollectorName: feeCollectorName,

		bankK: bankK,
		accK:  accK,

		params: collections.NewItem(
			sb,
			types.ParamsKey,
			"parameters",
			codec.CollValue[types.Params](cdc),
		),
		currentRewards: collections.NewItem(
			sb,
			types.CurrentRewardsKeyPrefix,
			"current_rewards",
			codec.CollValue[types.CurrentRewards](cdc),
		),
		historicalRewards: collections.NewMap(
			sb,
			types.HistoricalRewardsKeyPrefix,
			"historical_rewards",
			// key: (period)
			collections.Uint64Key,
			codec.CollValue[types.HistoricalRewards](cdc),
		),
		coostakerRewardsTracker: collections.NewMap(
			sb,
			types.CoostakerRewardsTrackerKeyPrefix,
			"coostaker_rewards_tracker",
			// key: (coostakerAddr)
			collections.BytesKey,
			codec.CollValue[types.CoostakerRewardsTracker](cdc),
		),
	}
}

func (k Keeper) Logger(goCtx context.Context) log.Logger {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
