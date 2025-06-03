package keeper

import (
	"context"

	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

type (
	Keeper struct {
		cdc              codec.BinaryCodec
		storeService     corestoretypes.KVStoreService
		ics4Wrapper      types.ICS4Wrapper
		clientKeeper     types.ClientKeeper
		connectionKeeper types.ConnectionKeeper
		channelKeeper    types.ChannelKeeper
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
		// The address capable of executing a MsgUpdateParams message.
		// Typically, this should be the x/gov module account.
		authority string
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	ics4Wrapper types.ICS4Wrapper,
	clientKeeper types.ClientKeeper,
	connectionKeeper types.ConnectionKeeper,
	channelKeeper types.ChannelKeeper,
	authKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	btclcKeeper types.BTCLightClientKeeper,
	checkpointingKeeper types.CheckpointingKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	epochingKeeper types.EpochingKeeper,
	storeQuerier storetypes.Queryable,
	bsKeeper types.BTCStakingKeeper,
	btcStkKeeper types.BTCStkConsumerKeeper,
	authority string,
) *Keeper {
	return &Keeper{
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
		authority:           authority,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+ibcexported.ModuleName+"-"+types.ModuleName)
}

// GetPort returns the portID for the transfer module. Used in ExportGenesis
func (k Keeper) GetPort(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	port, err := store.Get(types.PortKey)
	if err != nil {
		panic(err)
	}
	return string(port)
}

// SetPort sets the portID for the transfer module. Used in InitGenesis
func (k Keeper) SetPort(ctx context.Context, portID string) {
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(types.PortKey, []byte(portID)); err != nil {
		panic(err)
	}
}
