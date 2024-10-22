package keeper

import (
	"testing"

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
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

func FinalityKeeperWithStore(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
	bsKeeper types.BTCStakingKeeper,
	iKeeper types.IncentiveKeeper,
	ckptKeeper types.CheckpointingKeeper,
) (*keeper.Keeper, sdk.Context) {
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		bsKeeper,
		iKeeper,
		ckptKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})

	return &k, ctx
}

func FinalityKeeper(
	t testing.TB,
	bsKeeper types.BTCStakingKeeper,
	iKeeper types.IncentiveKeeper,
	ckptKeeper types.CheckpointingKeeper,
) (*keeper.Keeper, sdk.Context) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	k, ctx := FinalityKeeperWithStore(t, db, stateStore, bsKeeper, iKeeper, ckptKeeper)

	// Initialize params
	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	return k, ctx
}
