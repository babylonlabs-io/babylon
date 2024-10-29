package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/btcdistribution/keeper"
	"github.com/babylonlabs-io/babylon/x/btcdistribution/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func NewKeeper(t *testing.T, btcStk types.BTCStakingKeeper, stk types.StakingKeeper) (keeper.Keeper, sdk.Context) {
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})

	return keeper.NewKeeper(btcStk, stk, runtime.NewKVStoreService(storeKey), cdc), ctx
}

func TestEndBlockerRewardsAccumulated(t *testing.T) {
	del1 := datagen.GenRandomAddress()
	del2 := datagen.GenRandomAddress()
	del3 := datagen.GenRandomAddress()

	/// table test with 3 dels

	// del1 7 btc 20 bbn
	// del2 3 btc 80 bbn
	// del3 15 btc 10 bbn
	// total 25 btc 110 bbn
	ctxB := context.Background()

	btcStkMap := map[string]math.Int{
		del1.String(): math.NewInt(7_00000000),
		del2.String(): math.NewInt(3_00000000),
		del3.String(): math.NewInt(15_00000000),
	}
	btcStk := NewMockBtcStk(btcStkMap)

	totalBtcStk, err := btcStk.TotalSatoshiStaked(ctxB)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(25_00000000).String(), totalBtcStk.String())

	stkMap := map[string]math.Int{
		del1.String(): math.NewInt(20_000000),
		del2.String(): math.NewInt(80_000000),
		del3.String(): math.NewInt(10_000000),
	}
	stk := NewMockStk(stkMap)

	totalNativeStk, err := stk.TotalBondedTokens(ctxB)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(110_000000).String(), totalNativeStk.String())

	k, ctx := NewKeeper(t, btcStk, stk)
	require.Equal(t, k.RewardsForCurrentBlock().String(), "10000000ubbn")

	err = k.EndBlocker(ctx)
	require.NoError(t, err)

	del1Rwd, err := k.GetDelRewards(ctx, del1)
	require.NoError(t, err)
	require.Equal(t, "", del1Rwd.String())

}
