package keeper

import (
	"testing"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"github.com/btcsuite/btcd/chaincfg"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v2/app/params"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
)

func BTCStakingKeeperWithStore(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
	storeKey *storetypes.KVStoreKey,
	btclcKeeper types.BTCLightClientKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	iKeeper types.IncentiveKeeper,
) (*keeper.Keeper, sdk.Context) {
	if storeKey == nil {
		storeKey = storetypes.NewKVStoreKey(types.StoreKey)
	}

	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		btclcKeeper,
		btccKeeper,
		iKeeper,
		&chaincfg.SimNetParams,
		appparams.AccBTCStaking.String(),
		appparams.AccGov.String(),
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})

	return &k, ctx
}

func BTCStakingKeeper(
	t testing.TB,
	btclcKeeper types.BTCLightClientKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	iKeeper types.IncentiveKeeper,
) (*keeper.Keeper, sdk.Context) {
	return BTCStakingKeeperWithStoreKey(t, nil, btclcKeeper, btccKeeper, iKeeper)
}

func BTCStakingKeeperWithStoreKey(
	t testing.TB,
	storeKey *storetypes.KVStoreKey,
	btclcKeeper types.BTCLightClientKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	iKeeper types.IncentiveKeeper,
) (*keeper.Keeper, sdk.Context) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	k, ctx := BTCStakingKeeperWithStore(t, db, stateStore, storeKey, btclcKeeper, btccKeeper, iKeeper)

	// Initialize params
	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	return k, ctx
}
