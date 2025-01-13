package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
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
		RefundableMsgKeySet collections.KeySet[[]byte]

		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string
		// name of the FeeCollector ModuleAccount
		feeCollectorName string

		// Collections structures for rewards

		// BTCDelegationRewardsTracker maps (FpAddr, DelAddr) => BTCDelegationRewardsTracker
		BTCDelegationRewardsTracker collections.Map[collections.Pair[[]byte, []byte], types.BTCDelegationRewardsTracker]
		// FinalityProviderHistoricalRewards maps (FpAddr, period) => FinalityProviderHistoricalRewards
		FinalityProviderHistoricalRewards collections.Map[collections.Pair[[]byte, uint64], types.FinalityProviderHistoricalRewards]
		// FinalityProviderCurrentRewards maps (FpAddr) => FinalityProviderCurrentRewards
		FinalityProviderCurrentRewards collections.Map[[]byte, types.FinalityProviderCurrentRewards]
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
		BTCDelegationRewardsTracker: collections.NewMap(
			sb,
			types.BTCDelegationRewardsTrackerKeyPrefix,
			"btc_delegation_rewards_tracker",
			// keys: (FpAddr, DelAddr)
			collections.PairKeyCodec(collections.BytesKey, collections.BytesKey),
			codec.CollValue[types.BTCDelegationRewardsTracker](cdc),
		),
		FinalityProviderHistoricalRewards: collections.NewMap(
			sb,
			types.FinalityProviderHistoricalRewardsKeyPrefix,
			"fp_historical_rewards",
			// keys: (FpAddr, period)
			collections.PairKeyCodec(collections.BytesKey, collections.Uint64Key),
			codec.CollValue[types.FinalityProviderHistoricalRewards](cdc),
		),
		FinalityProviderCurrentRewards: collections.NewMap(
			sb,
			types.FinalityProviderCurrentRewardsKeyPrefix,
			"fp_current_rewards",
			// key: (FpAddr)
			collections.BytesKey,
			codec.CollValue[types.FinalityProviderCurrentRewards](cdc),
		),
		authority:        authority,
		feeCollectorName: feeCollectorName,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
