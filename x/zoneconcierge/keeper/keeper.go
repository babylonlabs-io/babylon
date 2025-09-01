package keeper

import (
	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

type (
	Keeper struct {
		cdc              codec.BinaryCodec
		storeService     corestoretypes.KVStoreService
		ics4Wrapper      types.ICS4Wrapper
		clientKeeper     types.ClientKeeper
		connectionKeeper types.ConnectionKeeper
		channelKeeper    types.ZoneConciergeChannelKeeper
		authKeeper       types.AccountKeeper
		bankKeeper       types.BankKeeper
		// used in BTC timestamping
		btclcKeeper         types.BTCLightClientKeeper
		checkpointingKeeper types.CheckpointingKeeper
		btccKeeper          types.BtcCheckpointKeeper
		epochingKeeper      types.EpochingKeeper
		storeQuerier        storetypes.Queryable
		// used in BTC staking
		bsKeeper     types.BTCStakingKeeper
		btcStkKeeper types.BTCStkConsumerKeeper
		fKeeper      types.FinalityKeeper
		// The address capable of executing a MsgUpdateParams message.
		// Typically, this should be the x/gov module account.
		authority string

		// Transient store key for tracking BTC header and consumer event broadcasting triggers
		transientKey *storetypes.TransientStoreKey

		// Collections for KV store management
		Schema                collections.Schema
		ParamsCollection      collections.Item[types.Params]
		SealedEpochProof      collections.Map[uint64, types.ProofEpochSealed]
		BSNBTCState           collections.Map[string, types.BSNBTCState]
		LatestEpochHeaders    collections.Map[string, types.IndexedHeader]
		FinalizedEpochHeaders collections.Map[collections.Pair[uint64, string], types.IndexedHeaderWithProof]
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	transientKey *storetypes.TransientStoreKey,
	ics4Wrapper types.ICS4Wrapper,
	clientKeeper types.ClientKeeper,
	connectionKeeper types.ConnectionKeeper,
	channelKeeper types.ZoneConciergeChannelKeeper,
	authKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	btclcKeeper types.BTCLightClientKeeper,
	checkpointingKeeper types.CheckpointingKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	epochingKeeper types.EpochingKeeper,
	storeQuerier storetypes.Queryable,
	bsKeeper types.BTCStakingKeeper,
	btcStkKeeper types.BTCStkConsumerKeeper,
	fKeeper types.FinalityKeeper,
	authority string,
) *Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	k := &Keeper{
		cdc:                 cdc,
		storeService:        storeService,
		ics4Wrapper:         ics4Wrapper,
		clientKeeper:        clientKeeper,
		connectionKeeper:    connectionKeeper,
		channelKeeper:       channelKeeper,
		authKeeper:          authKeeper,
		bankKeeper:          bankKeeper,
		btclcKeeper:         btclcKeeper,
		checkpointingKeeper: checkpointingKeeper,
		btccKeeper:          btccKeeper,
		epochingKeeper:      epochingKeeper,
		storeQuerier:        storeQuerier,
		bsKeeper:            bsKeeper,
		btcStkKeeper:        btcStkKeeper,
		fKeeper:             fKeeper,
		authority:           authority,
		transientKey:        transientKey,

		ParamsCollection: collections.NewItem[types.Params](
			sb,
			types.ParamsKey,
			"params",
			codec.CollValue[types.Params](cdc),
		),
		SealedEpochProof: collections.NewMap[uint64, types.ProofEpochSealed](
			sb,
			types.SealedEpochProofKey,
			"sealed_epoch_proof",
			collections.Uint64Key,
			codec.CollValue[types.ProofEpochSealed](cdc),
		),
		BSNBTCState: collections.NewMap[string, types.BSNBTCState](
			sb,
			types.BSNBTCStateKey,
			"bsn_btc_state",
			collections.StringKey,
			codec.CollValue[types.BSNBTCState](cdc),
		),
		LatestEpochHeaders: collections.NewMap[string, types.IndexedHeader](
			sb,
			types.LatestEpochHeadersKey,
			"latest_epoch_headers",
			collections.StringKey,
			codec.CollValue[types.IndexedHeader](cdc),
		),
		FinalizedEpochHeaders: collections.NewMap[collections.Pair[uint64, string], types.IndexedHeaderWithProof](
			sb,
			types.FinalizedEpochHeadersKey,
			"finalized_epoch_headers",
			collections.PairKeyCodec(collections.Uint64Key, collections.StringKey),
			codec.CollValue[types.IndexedHeaderWithProof](cdc),
		),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+ibcexported.ModuleName+"-"+types.ModuleName)
}

func (k Keeper) GetPort() string {
	return types.PortID
}
