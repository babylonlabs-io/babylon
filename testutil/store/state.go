package store

import (
	"testing"

	"cosmossdk.io/core/header"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func NewStoreService(t *testing.T, moduleName string) (kvStore corestore.KVStoreService, stateStore storetypes.CommitMultiStore) {
	db := dbm.NewMemDB()
	stateStore = store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	storeKey := storetypes.NewKVStoreKey(moduleName)

	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	return runtime.NewKVStoreService(storeKey), stateStore
}

func NewStoreWithCtx(t *testing.T, moduleName string) (ctx sdk.Context, kvStore corestore.KVStoreService) {
	kvStore, stateStore := NewStoreService(t, moduleName)
	ctx = sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})
	return ctx, kvStore
}
