package keeper

import (
	"testing"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonchain/babylon/x/btcstaking/keeper"
	"github.com/babylonchain/babylon/x/btcstaking/types"
	bsckeeper "github.com/babylonchain/babylon/x/btcstkconsumer/keeper"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	"github.com/btcsuite/btcd/chaincfg"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func BTCStakingKeeper(
	t testing.TB,
	btclcKeeper types.BTCLightClientKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	ckptKeeper types.CheckpointingKeeper,
) (*keeper.Keeper, *bsckeeper.Keeper, sdk.Context) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	// mount KV store for BTC staking consumer keeper
	bscStoreKey := storetypes.NewKVStoreKey(bsctypes.StoreKey)
	stateStore.MountStoreWithDB(bscStoreKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())
	// create BTC staking consumer keeper
	bscKeeper := bsckeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(bscStoreKey),
		nil,
		nil,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	// mount KV store for BTC staking keeper
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		btclcKeeper,
		btccKeeper,
		ckptKeeper,
		bscKeeper,
		&chaincfg.SimNetParams,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})

	// Initialize params
	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	return &k, &bscKeeper, ctx
}
