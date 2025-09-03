package v3rc4_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/collections"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocoded "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stkkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	v3rc4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc4"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	costkkeeper "github.com/babylonlabs-io/babylon/v4/x/coostaking/keeper"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

func setupTestKeepers(t *testing.T) (sdk.Context, codec.BinaryCodec, corestore.KVStoreService, *stkkeeper.Keeper, btcstkkeeper.Keeper, *costkkeeper.Keeper, *gomock.Controller) {
	ctrl := gomock.NewController(t)

	// Create DB and store
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	// Setup keepers
	btclcKeeper := btcstktypes.NewMockBTCLightClientKeeper(ctrl)
	btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 10}).AnyTimes()

	btccKeeper := btcstktypes.NewMockBtcCheckpointKeeper(ctrl)
	btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()

	distK := costktypes.NewMockDistributionKeeper(ctrl)

	btcStoreKey := storetypes.NewKVStoreKey(btcstktypes.StoreKey)
	btcStkKeeper, btcCtx := testutilkeeper.BTCStakingKeeperWithStore(t, db, stateStore, btcStoreKey, btclcKeeper, btccKeeper, nil, nil)

	accK := testutilkeeper.AccountKeeper(t, db, stateStore)
	bankKeeper := testutilkeeper.BankKeeper(t, db, stateStore, accK)
	stkKeeper := testutilkeeper.StakingKeeper(t, db, stateStore, accK, bankKeeper)
	// Setup coostaking store service
	costkStoreKey := storetypes.NewKVStoreKey(costktypes.StoreKey)
	costkKeeper, _ := testutilkeeper.CoostakingKeeperWithStore(t, db, stateStore, costkStoreKey, bankKeeper, accK, stkKeeper, distK)
	require.NoError(t, stateStore.LoadLatestVersion())
	costkStoreService := runtime.NewKVStoreService(costkStoreKey)

	// Setup codec
	registry := codectypes.NewInterfaceRegistry()
	cryptocoded.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	return btcCtx, cdc, costkStoreService, stkKeeper, *btcStkKeeper, costkKeeper, ctrl
}

func TestInitializeCoStakerRwdsTracker_EmptyState(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	// Test with empty state (no BTC stakers, no staking delegations)
	err := v3rc4.InitializeCoStakerRwdsTracker(
		ctx,
		cdc,
		storeService,
		stkKeeper,
		btcStkKeeper,
		*costkKeeper,
	)
	require.NoError(t, err)

	// Verify no co-staker rewards trackers were created
	count := countCoStakers(t, ctx, cdc, storeService)
	require.Equal(t, 0, count, "No co-staker rewards trackers should exist in empty state")
}

func TestInitializeCoStakerRwdsTracker_WithRealDelegations(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegation
	btcDel := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// Create baby staking delegation
	babyAmount := math.NewInt(25000)
	createBabyDelegation(t, ctx, stkKeeper, stakerAddr, babyAmount)

	// Execute upgrade function
	err := v3rc4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created
	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, math.NewIntFromUint64(btcDel.TotalSat), babyAmount)
}

func TestInitializeCoStakerRwdsTracker_OnlyBTCStaking(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegation directly in keeper (no staking delegation)
	createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// Execute upgrade function
	err := v3rc4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper,
	)
	require.NoError(t, err)

	// Verify NO co-staker was created (BTC only, no baby staking)
	verifyNoCoStakerCreated(t, ctx, cdc, storeService, stakerAddr)
}

func TestInitializeCoStakerRwdsTracker_MultipleCombinations(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Case 1: BTC + Baby staking (should create co-staker)
	staker1Addr := datagen.GenRandomAccount().GetAddress()
	btcDel1 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker1Addr, 60000)

	// Case 2: Only BTC staking (should NOT create co-staker)
	staker2Addr := datagen.GenRandomAccount().GetAddress()
	createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker2Addr, 80000)

	// Case 3: BTC + Baby staking (should create co-staker)
	staker3Addr := datagen.GenRandomAccount().GetAddress()
	btcDel3 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker3Addr, 100000)

	// create staking delegations - only staker1 and staker3 have baby staking
	babyDel1Amt := math.NewInt(30000)
	createBabyDelegation(t, ctx, stkKeeper, staker1Addr, babyDel1Amt)

	babyDel3Amt := math.NewInt(50000)
	createBabyDelegation(t, ctx, stkKeeper, staker3Addr, babyDel3Amt)

	// Execute upgrade function
	err := v3rc4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper,
	)
	require.NoError(t, err)

	// Verify results
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker1Addr, math.NewIntFromUint64(btcDel1.TotalSat), babyDel1Amt) // Co-staker
	verifyNoCoStakerCreated(t, ctx, cdc, storeService, staker2Addr)                                                     // BTC only
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker3Addr, math.NewIntFromUint64(btcDel3.TotalSat), babyDel3Amt) // Co-staker

	// Verify total count
	count := countCoStakers(t, ctx, cdc, storeService)
	require.Equal(t, 2, count, "Should have exactly 2 co-stakers created")
}

func TestInitializeCoStakerRwdsTracker_MultipleStakingFromSameStaker(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create multiple BTC delegations for the same staker
	btcDel1 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 30000) // 30k sats
	btcDel2 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 20000) // 20k sats
	btcDel3 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 25000) // 25k sats
	// Total BTC: 75k sats

	// Create multiple baby delegations for the same staker
	babyDel1Amt := math.NewInt(10000)
	createBabyDelegation(t, ctx, stkKeeper, stakerAddr, babyDel1Amt)

	babyDel2Amt := math.NewInt(15000)
	createBabyDelegation(t, ctx, stkKeeper, stakerAddr, babyDel2Amt)

	babyDel3Amt := math.NewInt(12000)
	createBabyDelegation(t, ctx, stkKeeper, stakerAddr, babyDel3Amt)
	// Total Baby: 37k tokens

	// Execute upgrade function
	err := v3rc4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created with accumulated amounts
	expectedTotalBTC := math.NewIntFromUint64(btcDel1.TotalSat + btcDel2.TotalSat + btcDel3.TotalSat) // 75k sats
	expectedTotalBaby := babyDel1Amt.Add(babyDel2Amt).Add(babyDel3Amt)                                // 37k tokens

	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, expectedTotalBTC, expectedTotalBaby)

	// Verify total count is 1 (only one unique staker)
	count := countCoStakers(t, ctx, cdc, storeService)
	require.Equal(t, 1, count, "Should have exactly 1 co-staker created despite multiple delegations")
}

// Helper functions

func createTestBTCDelegation(t *testing.T, r *rand.Rand, ctx sdk.Context, btcStkKeeper btcstkkeeper.Keeper, stakerAddr sdk.AccAddress, stakingValue uint64) *btcstktypes.BTCDelegation {
	// Generate random BTC keys
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	// Generate finality provider
	fp, err := datagen.GenRandomFinalityProvider(r, "", "")
	require.NoError(t, err)
	// Create BTC delegation
	startHeight := uint32(10)
	endHeight := uint32(datagen.RandomInt(r, 1000)) + startHeight + btcctypes.DefaultParams().CheckpointFinalizationTimeout + 1
	stakingTime := endHeight - startHeight
	slashingRate := math.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)
	slashingChangeLockTime := uint16(101)
	slashingAddress, err := datagen.GenRandomBTCAddress(r, &chaincfg.RegressionNetParams)
	require.NoError(t, err)
	slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
	require.NoError(t, err)

	covenantSKs, covenantPKs, covenantQuorum := datagen.GenCovenantCommittee(r)
	del, err := datagen.GenRandomBTCDelegation(
		r,
		t,
		&chaincfg.RegressionNetParams,
		[]bbn.BIP340PubKey{*fp.BtcPk},
		delSK,
		"",
		covenantSKs,
		covenantPKs,
		covenantQuorum,
		slashingPkScript,
		stakingTime, startHeight, endHeight, stakingValue,
		slashingRate,
		slashingChangeLockTime,
	)
	if err != nil {
		panic(err)
	}

	// Set staker address
	del.StakerAddr = stakerAddr.String()
	del.TotalSat = stakingValue

	require.NoError(t, btcStkKeeper.AddBTCDelegation(ctx, del))

	return del
}

func createBabyDelegation(t *testing.T, ctx context.Context, stkKeeper *stkkeeper.Keeper, stakerAddr sdk.AccAddress, delAmount math.Int) {
	validatorAddr := datagen.GenRandomValidatorAddress()
	delegation := stktypes.Delegation{
		DelegatorAddress: stakerAddr.String(),
		ValidatorAddress: validatorAddr.String(),
		Shares:           math.LegacyNewDecFromInt(delAmount),
	}

	require.NoError(t, stkKeeper.SetValidator(ctx, stktypes.Validator{
		OperatorAddress: validatorAddr.String(),
		Tokens:          delAmount,
		DelegatorShares: math.LegacyNewDecFromInt(delAmount),
	}))
	require.NoError(t, stkKeeper.SetDelegation(ctx, delegation))
}

func verifyCoStakerCreated(t *testing.T, ctx sdk.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService, stakerAddr sdk.AccAddress, expectedBTCAmount, expectedBabyAmount math.Int) {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	tracker, err := rwdTrackers.Get(ctx, []byte(stakerAddr))

	require.NoError(t, err, "Co-staker rewards tracker should exist for %s", stakerAddr.String())
	require.Equal(t, uint64(1), tracker.StartPeriodCumulativeReward, "StartPeriodCumulativeReward should be 1")
	require.True(t, tracker.ActiveSatoshis.Equal(expectedBTCAmount), "ActiveSatoshis should match expected BTC amount: expected %s, got %s", expectedBTCAmount.String(), tracker.ActiveSatoshis.String())
	require.True(t, tracker.ActiveBaby.Equal(expectedBabyAmount), "ActiveBaby should match expected baby amount: expected %s, got %s", expectedBabyAmount.String(), tracker.ActiveBaby.String())
}

func verifyNoCoStakerCreated(t *testing.T, ctx sdk.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService, stakerAddr sdk.AccAddress) {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	_, err := rwdTrackers.Get(ctx, []byte(stakerAddr))
	require.Error(t, err, "Co-staker rewards tracker should not exist for %s", stakerAddr.String())
	require.ErrorIs(t, err, collections.ErrNotFound)
}

func countCoStakers(t *testing.T, ctx sdk.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService) int {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	var count int
	err := rwdTrackers.Walk(ctx, nil, func(key []byte, value costktypes.CoostakerRewardsTracker) (stop bool, err error) {
		count++
		return false, nil
	})
	require.NoError(t, err)
	return count
}

func rwdTrackerCollection(storeService corestore.KVStoreService, cdc codec.BinaryCodec) collections.Map[[]byte, costktypes.CoostakerRewardsTracker] {
	sb := collections.NewSchemaBuilder(storeService)
	rwdTrackers := collections.NewMap(
		sb,
		costktypes.CoostakerRewardsTrackerKeyPrefix,
		"coostaker_rewards_tracker",
		collections.BytesKey,
		codec.CollValue[costktypes.CoostakerRewardsTracker](cdc),
	)
	return rwdTrackers
}
