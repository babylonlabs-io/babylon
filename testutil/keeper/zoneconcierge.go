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
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

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

func ZoneConciergeKeeper(
	t testing.TB,
	channelKeeper types.ChannelKeeper,
	btclcKeeper types.BTCLightClientKeeper,
	checkpointingKeeper types.CheckpointingKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	epochingKeeper types.EpochingKeeper,
	bsKeeper types.BTCStakingKeeper,
	btcStkKeeper types.BTCStkConsumerKeeper,
) (*keeper.Keeper, sdk.Context) {
	return ZoneConciergeKeeperWithStoreKey(
		t,
		nil,
		channelKeeper,
		btclcKeeper,
		checkpointingKeeper,
		btccKeeper,
		epochingKeeper,
		bsKeeper,
		btcStkKeeper,
	)
}

func ZoneConciergeKeeperWithStoreKey(
	t testing.TB,
	storeKey *storetypes.KVStoreKey,
	channelKeeper types.ChannelKeeper,
	btclcKeeper types.BTCLightClientKeeper,
	checkpointingKeeper types.CheckpointingKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	epochingKeeper types.EpochingKeeper,
	bsKeeper types.BTCStakingKeeper,
	btcStkKeeper types.BTCStkConsumerKeeper,
) (*keeper.Keeper, sdk.Context) {
	logger := log.NewTestLogger(t)
	if storeKey == nil {
		storeKey = storetypes.NewKVStoreKey(types.StoreKey)
	}
	memStoreKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, logger, metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memStoreKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	appCodec := codec.NewProtoCodec(registry)

	k := keeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(storeKey),
		nil, // TODO: mock this keeper
		nil, // TODO: mock this keeper
		nil, // TODO: mock this keeper
		keeper.NewChannelKeeper(appCodec, runtime.NewKVStoreService(storeKey), channelKeeper),
		nil, // TODO: mock this keeper
		nil, // TODO: mock this keeper
		btclcKeeper,
		checkpointingKeeper,
		btccKeeper,
		epochingKeeper,
		zoneconciergeStoreQuerier{},
		bsKeeper,
		btcStkKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, logger)
	ctx = ctx.WithHeaderInfo(header.Info{})

	return k, ctx
}
