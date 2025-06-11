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
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
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
		appparams.AccGov.String(),
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
	dParams := types.DefaultParams()
	dParams.FinalityActivationHeight = 0
	if err := k.SetParams(ctx, dParams); err != nil {
		panic(err)
	}

	return k, ctx
}
