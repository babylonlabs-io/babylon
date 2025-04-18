package keeper

import (
	"math/big"
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

	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/x/btccheckpoint/keeper"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
)

func NewBTCCheckpointKeeper(
	t testing.TB,
	lk btcctypes.BTCLightClientKeeper,
	ek btcctypes.CheckpointingKeeper,
	ik btcctypes.IncentiveKeeper,
	powLimit *big.Int,
) (*keeper.Keeper, sdk.Context) {
	return NewBTCChkptKeeperWithStoreKeys(t, nil, nil, lk, ek, ik, powLimit)
}

func NewBTCChkptKeeperWithStoreKeys(
	t testing.TB,
	storeKey *storetypes.KVStoreKey,
	tstoreKey *storetypes.TransientStoreKey,
	lk btcctypes.BTCLightClientKeeper,
	ek btcctypes.CheckpointingKeeper,
	ik btcctypes.IncentiveKeeper,
	powLimit *big.Int,
) (*keeper.Keeper, sdk.Context) {
	if storeKey == nil {
		storeKey = storetypes.NewKVStoreKey(btcctypes.StoreKey)
	}
	if tstoreKey == nil {
		tstoreKey = storetypes.NewTransientStoreKey(btcctypes.TStoreKey)
	}

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		tstoreKey,
		lk,
		ek,
		ik,
		powLimit,
		appparams.AccGov.String(),
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})

	// Initialize params
	if err := k.SetParams(ctx, btcctypes.DefaultParams()); err != nil {
		panic(err)
	}

	return &k, ctx
}
