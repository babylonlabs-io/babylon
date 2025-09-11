package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService corestoretypes.KVStoreService

		bankKeeper     types.BankKeeper
		accountKeeper  types.AccountKeeper
		epochingKeeper types.EpochingKeeper

		// RefundableMsgKeySet is the set of hashes of messages that can be refunded
		// Each key is a hash of the message bytes
		// TODO: remove it after deciding on removing refundable_msg_hashes in the genesis.proto
		RefundableMsgKeySet collections.KeySet[[]byte]

		// RefundableMsgCount is the counter for refundable msgs only maintained per-tx
		RefundableMsgCount int64

		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string
		// name of the FeeCollector ModuleAccount
		feeCollectorName string

		// Collections structures for rewards

		// btcDelegationRewardsTracker maps (FpAddr, DelAddr) => btcDelegationRewardsTracker
		btcDelegationRewardsTracker collections.Map[collections.Pair[[]byte, []byte], types.BTCDelegationRewardsTracker]
		// finalityProviderHistoricalRewards maps (FpAddr, period) => finalityProviderHistoricalRewards
		finalityProviderHistoricalRewards collections.Map[collections.Pair[[]byte, uint64], types.FinalityProviderHistoricalRewards]
		// finalityProviderCurrentRewards maps (FpAddr) => finalityProviderCurrentRewards
		finalityProviderCurrentRewards collections.Map[[]byte, types.FinalityProviderCurrentRewards]

		// rewardTrackerEvents maps (babylon block height) => types.EventsPowerUpdateAtHeight
		rewardTrackerEvents collections.Map[uint64, types.EventsPowerUpdateAtHeight]
		// rewardTrackerEventsLastProcessedHeight keeps track of the latest processed babylon block height of events
		rewardTrackerEventsLastProcessedHeight collections.Item[uint64]
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
	epochingKeeper types.EpochingKeeper,
	authority string,
	feeCollectorName string,
) Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	return Keeper{
		cdc:            cdc,
		storeService:   storeService,
		bankKeeper:     bankKeeper,
		accountKeeper:  accountKeeper,
		epochingKeeper: epochingKeeper,
		RefundableMsgKeySet: collections.NewKeySet(
			sb,
			types.RefundableMsgKeySetPrefix,
			"refundable_msg_key_set",
			collections.BytesKey,
		),

		// Collections structures for rewards
		btcDelegationRewardsTracker: collections.NewMap(
			sb,
			types.BTCDelegationRewardsTrackerKeyPrefix,
			"btc_delegation_rewards_tracker",
			// keys: (FpAddr, DelAddr)
			collections.PairKeyCodec(collections.BytesKey, collections.BytesKey),
			codec.CollValue[types.BTCDelegationRewardsTracker](cdc),
		),
		finalityProviderHistoricalRewards: collections.NewMap(
			sb,
			types.FinalityProviderHistoricalRewardsKeyPrefix,
			"fp_historical_rewards",
			// keys: (FpAddr, period)
			collections.PairKeyCodec(collections.BytesKey, collections.Uint64Key),
			codec.CollValue[types.FinalityProviderHistoricalRewards](cdc),
		),
		finalityProviderCurrentRewards: collections.NewMap(
			sb,
			types.FinalityProviderCurrentRewardsKeyPrefix,
			"fp_current_rewards",
			// key: (FpAddr)
			collections.BytesKey,
			codec.CollValue[types.FinalityProviderCurrentRewards](cdc),
		),
		rewardTrackerEvents: collections.NewMap(
			sb,
			types.RewardTrackerEvents,
			"events_reward_tracker",
			// key: (babylon block height)
			collections.Uint64Key,
			codec.CollValue[types.EventsPowerUpdateAtHeight](cdc),
		),
		rewardTrackerEventsLastProcessedHeight: collections.NewItem[uint64](
			sb,
			types.RewardTrackerEventsLastProcessedHeight,
			"last_processed_height_events_reward_tracker",
			collections.Uint64Value,
		),
		authority:        authority,
		feeCollectorName: feeCollectorName,
	}
}

func (k Keeper) Logger(goCtx context.Context) log.Logger {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
