package keeper

import (
	"testing"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
)

// zoneconciergeChannelKeeper is a stub of ChannelKeeper
type zoneconciergeChannelKeeper struct{}

func (zoneconciergeChannelKeeper) GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool) {
	return channeltypes.Channel{}, false
}
func (zoneconciergeChannelKeeper) GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool) {
	return 0, false
}
func (zoneconciergeChannelKeeper) SendPacket(ctx sdk.Context, channelCap *capabilitytypes.Capability, packet ibcexported.PacketI) error {
	return nil
}
func (zoneconciergeChannelKeeper) ChanCloseInit(ctx sdk.Context, portID, channelID string, chanCap *capabilitytypes.Capability) error {
	return nil
}

func (zoneconciergeChannelKeeper) GetAllChannels(ctx sdk.Context) []channeltypes.IdentifiedChannel {
	return nil
}
func (zoneconciergeChannelKeeper) GetChannelClientState(ctx sdk.Context, portID, channelID string) (string, ibcexported.ClientState, error) {
	return "", nil, nil
}

// zoneconciergeportKeeper is a stub of PortKeeper
type zoneconciergePortKeeper struct{}

func (zoneconciergePortKeeper) BindPort(ctx sdk.Context, portID string) *capabilitytypes.Capability {
	return &capabilitytypes.Capability{}
}

type zoneconciergeStoreQuerier struct{}

func (zoneconciergeStoreQuerier) Query(req *storetypes.RequestQuery) (*storetypes.ResponseQuery, error) {
	return &storetypes.ResponseQuery{
		ProofOps: &cmtcrypto.ProofOps{
			Ops: []cmtcrypto.ProofOp{
				cmtcrypto.ProofOp{},
			},
		},
	}, nil
}

func ZoneConciergeKeeper(t testing.TB, btclcKeeper types.BTCLightClientKeeper, checkpointingKeeper types.CheckpointingKeeper, btccKeeper types.BtcCheckpointKeeper, epochingKeeper types.EpochingKeeper) (*keeper.Keeper, sdk.Context) {
	logger := log.NewTestLogger(t)
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	memStoreKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, logger, metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memStoreKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	appCodec := codec.NewProtoCodec(registry)
	capabilityKeeper := capabilitykeeper.NewKeeper(appCodec, storeKey, memStoreKey)
	k := keeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(storeKey),
		nil, // TODO: mock this keeper
		nil, // TODO: mock this keeper
		zoneconciergeChannelKeeper{},
		zoneconciergePortKeeper{},
		nil, // TODO: mock this keeper
		nil, // TODO: mock this keeper
		btclcKeeper,
		checkpointingKeeper,
		btccKeeper,
		epochingKeeper,
		zoneconciergeStoreQuerier{},
		capabilityKeeper.ScopeToModule("ZoneconciergeScopedKeeper"),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, logger)
	ctx = ctx.WithHeaderInfo(header.Info{})

	return k, ctx
}
