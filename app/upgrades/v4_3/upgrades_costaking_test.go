package v4_3_test

import (
	"math/rand"
	"testing"
	"time"

	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v4_3 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_3"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	tkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	costkkeeper "github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	epochkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	stkkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
)

func setupTestKeepers(t *testing.T) (sdk.Context, codec.BinaryCodec, corestore.KVStoreService, costkkeeper.Keeper, *stkkeeper.Keeper, epochkeeper.Keeper, *gomock.Controller) {
	ctrl := gomock.NewController(t)

	// Create DB and store
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	distK := costktypes.NewMockDistributionKeeper(ctrl)

	// Setup keepers
	accK := testutilkeeper.AccountKeeper(t, db, stateStore)
	bankKeeper := testutilkeeper.BankKeeper(t, db, stateStore, accK)
	stkKeeper := testutilkeeper.StakingKeeper(t, db, stateStore, accK, bankKeeper)
	incentiveK, incentiveCtx := testutilkeeper.IncentiveKeeperWithStore(t, db, stateStore, nil, bankKeeper, accK, nil)

	// Create costaking module account
	costkModuleAcc := authtypes.NewEmptyModuleAccount(costktypes.ModuleName)
	accK.SetModuleAccount(incentiveCtx, costkModuleAcc)

	// Setup codec with all registrations
	encCfg := appparams.DefaultEncodingConfig()
	cdc := encCfg.Codec

	// Setup costaking store service and keeper
	costkStoreKey := storetypes.NewKVStoreKey(costktypes.StoreKey)
	costkKeeper, ctx := testutilkeeper.CostakingKeeperWithStore(t, db, stateStore, costkStoreKey, bankKeeper, accK, incentiveK, stkKeeper, distK)

	require.NoError(t, stateStore.LoadLatestVersion())
	costkStoreService := runtime.NewKVStoreService(costkStoreKey)

	epochingK := testutilkeeper.EpochingKeeperWithStore(t, db, stateStore, nil, bankKeeper, stkKeeper)
	err := epochingK.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)

	dftBondDenoms := stktypes.DefaultParams()
	dftBondDenoms.BondDenom = appparams.DefaultBondDenom
	err = stkKeeper.SetParams(ctx, dftBondDenoms)
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(10)

	return ctx, cdc, costkStoreService, *costkKeeper, stkKeeper, *epochingK, ctrl
}

func TestResetCoStakerRwdsTracker_WithPreexistingTrackers(t *testing.T) {
	ctx, cdc, storeService, costkK, stkK, epochingK, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, costkK.SetParams(ctx, costktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	stakerAddr := datagen.GenRandomAccount().GetAddress()
	startPeriod := uint64(5)

	rndAmtBaby := datagen.RandomMathInt(r, 1000).AddRaw(2)
	rndAmtSats := datagen.RandomMathInt(r, 1000).AddRaw(10)
	correctAmtBaby := rndAmtBaby.AddRaw(5)

	tkeeper.CreateCostakerRewardsTracker(t, ctx, cdc, storeService, stakerAddr, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: startPeriod,
		ActiveSatoshis:              rndAmtSats,
		ActiveBaby:                  rndAmtBaby,
	})

	pks := simtestutil.CreateTestPubKeys(5)
	valAddr := datagen.GenRandomValidatorAddress()
	validator := testutil.NewValidator(t, valAddr, pks[0])

	err := stkK.SetValidator(ctx, validator)
	require.NoError(t, err)

	rEpoch := datagen.GenRandomEpoch(r)
	rEpoch.EpochNumber = 1
	err = epochingK.InitEpoch(ctx, []*types.Epoch{
		rEpoch,
	})
	require.NoError(t, err)

	err = epochingK.InitGenValidatorSet(ctx, []*types.EpochValidatorSet{
		&types.EpochValidatorSet{
			EpochNumber: 1,
			Validators: []*types.Validator{
				{
					Addr:  valAddr,
					Power: 10,
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = stkK.Delegate(ctx, stakerAddr, correctAmtBaby, stktypes.Unbonded, validator, false)
	require.NoError(t, err)

	err = v4_3.ResetCoStakerRwdsTrackerActiveBaby(ctx, cdc, storeService, epochingK, stkK, costkK)
	require.NoError(t, err)

	costkRwd, err := costkK.GetCostakerRewards(ctx, stakerAddr)
	require.NoError(t, err)
	require.Equal(t, costkRwd.ActiveBaby.String(), correctAmtBaby.String())
	require.Equal(t, costkRwd.ActiveSatoshis.String(), rndAmtSats.String())

	costkP := costkK.GetParams(ctx)
	expScore := costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, correctAmtBaby, rndAmtSats)
	require.Equal(t, costkRwd.TotalScore.String(), expScore.String())
}
