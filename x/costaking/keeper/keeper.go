package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
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

		bankK  types.BankKeeper
		accK   types.AccountKeeper
		ictvK  types.IncentiveKeeper
		stkK   types.StakingKeeper
		distrK types.DistributionKeeper

		// cache for delta changes in baby delegations
		stkCache *types.StakingCache

		// params stores the module parameter
		params collections.Item[types.Params]

		// Collections structures for rewards

		// currentRewards stores the current rewards information
		currentRewards collections.Item[types.CurrentRewards]
		// historicalRewards maps (period) => historicalRewards
		historicalRewards collections.Map[uint64, types.HistoricalRewards]
		// costakerRewardsTracker maps (costakerAddr) => costakerRewardsTracker
		costakerRewardsTracker collections.Map[[]byte, types.CostakerRewardsTracker]
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	bankK types.BankKeeper,
	accK types.AccountKeeper,
	ictvK types.IncentiveKeeper,
	stkK types.StakingKeeper,
	distrK types.DistributionKeeper,
	authority string,
	feeCollectorName string,
) Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	return Keeper{
		cdc:          cdc,
		storeService: storeService,

		authority:        authority,
		feeCollectorName: feeCollectorName,

		bankK:  bankK,
		accK:   accK,
		ictvK:  ictvK,
		stkK:   stkK,
		distrK: distrK,

		stkCache: types.NewStakingCache(),

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
		costakerRewardsTracker: collections.NewMap(
			sb,
			types.CostakerRewardsTrackerKeyPrefix,
			"costaker_rewards_tracker",
			// key: (costakrAddr)
			collections.BytesKey,
			codec.CollValue[types.CostakerRewardsTracker](cdc),
		),
	}
}

func (k Keeper) Logger(goCtx context.Context) log.Logger {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// EndBlock clears the staking cache at the end of each block
func (k Keeper) EndBlock(ctx context.Context) error {
	k.stkCache.Clear()
	return nil
}
