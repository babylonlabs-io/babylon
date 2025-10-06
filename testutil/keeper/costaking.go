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
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func CostakingKeeperWithStore(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
	storeKey *storetypes.KVStoreKey,
	bankK types.BankKeeper,
	accK types.AccountKeeper,
	ictvK types.IncentiveKeeper,
	stkK types.StakingKeeper,
	distK types.DistributionKeeper,
	epochK types.EpochingKeeper,
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
		bankK,
		accK,
		ictvK,
		stkK,
		distK,
		epochK,
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

func CostakingKeeperWithMocks(t testing.TB, ctrl *gomock.Controller) (*keeper.Keeper, *gomock.Controller, sdk.Context) {
	if ctrl == nil {
		ctrl = gomock.NewController(t)
	}

	accK := types.NewMockAccountKeeper(ctrl)
	accK.EXPECT().GetModuleAddress(gomock.Any()).Return(authtypes.NewModuleAddress(types.ModuleName)).AnyTimes()

	k, ctx := CostakingKeeperWithStoreKey(t, nil, types.NewMockBankKeeper(ctrl), accK, types.NewMockIncentiveKeeper(ctrl), types.NewMockStakingKeeper(ctrl), types.NewMockDistributionKeeper(ctrl), types.NewMockEpochingKeeper(ctrl))
	return k, ctrl, ctx
}

func CostakingKeeperWithStoreKey(
	t testing.TB,
	storeKey *storetypes.KVStoreKey,
	bankK types.BankKeeper,
	accK types.AccountKeeper,
	ictvK types.IncentiveKeeper,
	stkK types.StakingKeeper,
	distK types.DistributionKeeper,
	epochK types.EpochingKeeper,
) (*keeper.Keeper, sdk.Context) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	k, ctx := CostakingKeeperWithStore(t, db, stateStore, storeKey, bankK, accK, ictvK, stkK, distK, epochK)

	// Initialize params
	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	return k, ctx
}
