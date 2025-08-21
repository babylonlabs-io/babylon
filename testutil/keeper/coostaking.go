package keeper

import (
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

func CoostakingKeeperWithStore(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
	storeKey *storetypes.KVStoreKey,
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
		appparams.AccGov.String(),
		authtypes.FeeCollectorName,
	)

	ctx := sdk.NewContext(
		stateStore,
		cmtproto.Header{
			Time: time.Now().UTC(),
		},
		false,
		log.NewNopLogger(),
	)
	ctx = ctx.WithHeaderInfo(header.Info{})

	return &k, ctx
}

func CoostakingKeeper(t testing.TB) (*keeper.Keeper, sdk.Context) {
	return CoostakingKeeperWithStoreKey(t, nil)
}

func CoostakingKeeperWithStoreKey(
	t testing.TB,
	storeKey *storetypes.KVStoreKey,
) (*keeper.Keeper, sdk.Context) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	k, ctx := CoostakingKeeperWithStore(t, db, stateStore, storeKey)

	// Initialize params
	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	return k, ctx
}
