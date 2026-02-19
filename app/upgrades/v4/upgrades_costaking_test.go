package v4_test

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
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stkkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	v4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	costkkeeper "github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	fkeeper "github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

func setupTestKeepers(t *testing.T) (sdk.Context, codec.BinaryCodec, corestore.KVStoreService, *stkkeeper.Keeper, btcstkkeeper.Keeper, *costkkeeper.Keeper, *fkeeper.Keeper, *gomock.Controller) {
	ctrl := gomock.NewController(t)

	// Create DB and store
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	// Setup mocked keepers
	btclcKeeper := btcstktypes.NewMockBTCLightClientKeeper(ctrl)
	btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 10}).AnyTimes()

	btccKeeper := btcstktypes.NewMockBtcCheckpointKeeper(ctrl)
	btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()

	distK := costktypes.NewMockDistributionKeeper(ctrl)

	btcStkStoreKey := storetypes.NewKVStoreKey(btcstktypes.StoreKey)
	btcStkKeeper, btcCtx := testutilkeeper.BTCStakingKeeperWithStore(t, db, stateStore, btcStkStoreKey, btclcKeeper, btccKeeper, nil)

	// Setup keepers
	accK := testutilkeeper.AccountKeeper(t, db, stateStore)
	bankKeeper := testutilkeeper.BankKeeper(t, db, stateStore, accK)

	// Create costaking module account
	costkModuleAcc := authtypes.NewEmptyModuleAccount(costktypes.ModuleName)
	accK.SetModuleAccount(btcCtx, costkModuleAcc)
	stkKeeper := testutilkeeper.StakingKeeper(t, db, stateStore, accK, bankKeeper)
	incentiveK, _ := testutilkeeper.IncentiveKeeperWithStore(t, db, stateStore, nil, bankKeeper, accK, nil)
	fKeeper, _ := testutilkeeper.FinalityKeeperWithStore(t, db, stateStore, btcStkKeeper, incentiveK, ftypes.NewMockCheckpointingKeeper(ctrl), ftypes.NewMockFinalityHooks(ctrl))

	// Setup costaking store service and keeper
	costkStoreKey := storetypes.NewKVStoreKey(costktypes.StoreKey)
	costkKeeper, _ := testutilkeeper.CostakingKeeperWithStore(t, db, stateStore, costkStoreKey, bankKeeper, accK, incentiveK, stkKeeper, distK)
	require.NoError(t, stateStore.LoadLatestVersion())
	costkStoreService := runtime.NewKVStoreService(costkStoreKey)

	// Setup codec
	registry := codectypes.NewInterfaceRegistry()
	cryptocoded.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	btcCtx = btcCtx.WithBlockHeight(10)

	return btcCtx, cdc, costkStoreService, stkKeeper, *btcStkKeeper, costkKeeper, fKeeper, ctrl
}

func TestInitializeCoStakerRwdsTracker_EmptyState(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	vp, _, err := datagen.GenRandomVotingPowerDistCache(r, 10)
	require.NoError(t, err)
	require.NotEmpty(t, vp.GetActiveFinalityProviderSet())
	fKeeper.SetVotingPowerDistCache(ctx, uint64(ctx.HeaderInfo().Height)-1, vp)

	// Test with empty state (no BTC stakers, no staking delegations)
	err = v4.InitializeCoStakerRwdsTracker(
		ctx,
		cdc,
		storeService,
		stkKeeper,
		btcStkKeeper,
		*costkKeeper,
		*fKeeper,
	)
	require.NoError(t, err)

	// Verify no co-staker rewards trackers were created
	count := countCoStakers(t, ctx, cdc, storeService)
	require.Equal(t, 0, count, "No co-staker rewards trackers should exist in empty state")
}

func TestInitializeCoStakerRwdsTracker_WithoutPowerDistCache(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegation
	createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// Create baby staking delegation
	babyAmount := math.NewInt(25000)
	createBabyDelegation(t, ctx, stkKeeper, stakerAddr, babyAmount)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created with zero active sats (baby staking only)
	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, math.ZeroInt(), babyAmount)
}

func TestInitializeCoStakerRwdsTracker_FpNotActive(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegation
	createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// Create baby staking delegation
	babyAmount := math.NewInt(25000)
	createBabyDelegation(t, ctx, stkKeeper, stakerAddr, babyAmount)

	// seed voting power dist cache with different FP (not the one the staker is delegating to)
	vp, _, err := datagen.GenRandomVotingPowerDistCache(r, 10)
	require.NoError(t, err)
	require.NotEmpty(t, vp.GetActiveFinalityProviderSet())
	fKeeper.SetVotingPowerDistCache(ctx, uint64(ctx.HeaderInfo().Height)-1, vp)

	// Execute upgrade function
	err = v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created with zero active sats (baby staking only, FP not active)
	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, math.ZeroInt(), babyAmount)
}

func TestInitializeCoStakerRwdsTracker_ValidatorNotActive(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegation
	btcDel := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// Create baby staking delegation to an INACTIVE validator (not in LastValidatorPowers)
	validatorAddr := datagen.GenRandomValidatorAddress()
	babyAmount := math.NewInt(25000)
	delegation := stktypes.Delegation{
		DelegatorAddress: stakerAddr.String(),
		ValidatorAddress: validatorAddr.String(),
		Shares:           math.LegacyNewDecFromInt(babyAmount),
	}

	// Create validator but DON'T add to LastValidatorPowers (making it inactive)
	validator := stktypes.Validator{
		OperatorAddress: validatorAddr.String(),
		Tokens:          babyAmount,
		DelegatorShares: math.LegacyNewDecFromInt(babyAmount),
		Status:          stktypes.Unbonded, // Inactive validator
	}
	require.NoError(t, stkKeeper.SetValidator(ctx, validator))
	require.NoError(t, stkKeeper.SetDelegation(ctx, delegation))
	// NOTE: Not calling SetLastValidatorPower - validator is NOT in active set

	// seed voting power dist cache with FP as active
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, btcDel.FpBtcPkList)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created with zero active baby (validator not active, BTC only)
	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, math.NewIntFromUint64(btcDel.TotalSat), math.ZeroInt())
}

func TestInitializeCoStakerRwdsTracker_WithRealDelegations(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
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

	// seed voting power dist cache with FP as active (the one the staker is delegating to)
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, btcDel.FpBtcPkList)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created
	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, math.NewIntFromUint64(btcDel.TotalSat), babyAmount)
}

func TestInitializeCoStakerRwdsTracker_OnlyBTCStaking(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegation directly in keeper (no staking delegation)
	btcDel := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// seed voting power dist cache with FP as active (the one the staker is delegating to)
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, btcDel.FpBtcPkList)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created (BTC only, no baby staking)
	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, math.NewIntFromUint64(btcDel.TotalSat), math.ZeroInt())
}

func TestInitializeCoStakerRwdsTracker_MixedActiveInactiveValidators(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegation
	btcDel := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// Create delegation to ACTIVE validator
	activeValAddr := datagen.GenRandomValidatorAddress()
	activeBabyAmount := math.NewInt(15000)
	activeDelegation := stktypes.Delegation{
		DelegatorAddress: stakerAddr.String(),
		ValidatorAddress: activeValAddr.String(),
		Shares:           math.LegacyNewDecFromInt(activeBabyAmount),
	}
	activeValidator := stktypes.Validator{
		OperatorAddress: activeValAddr.String(),
		Tokens:          activeBabyAmount,
		DelegatorShares: math.LegacyNewDecFromInt(activeBabyAmount),
		Status:          stktypes.Bonded,
	}
	require.NoError(t, stkKeeper.SetValidator(ctx, activeValidator))
	require.NoError(t, stkKeeper.SetDelegation(ctx, activeDelegation))
	// Mark as active validator
	power := stkKeeper.TokensToConsensusPower(ctx, activeBabyAmount)
	require.NoError(t, stkKeeper.SetLastValidatorPower(ctx, activeValAddr, power))

	// Create delegation to INACTIVE validator
	inactiveValAddr := datagen.GenRandomValidatorAddress()
	inactiveBabyAmount := math.NewInt(10000)
	inactiveDelegation := stktypes.Delegation{
		DelegatorAddress: stakerAddr.String(),
		ValidatorAddress: inactiveValAddr.String(),
		Shares:           math.LegacyNewDecFromInt(inactiveBabyAmount),
	}
	inactiveValidator := stktypes.Validator{
		OperatorAddress: inactiveValAddr.String(),
		Tokens:          inactiveBabyAmount,
		DelegatorShares: math.LegacyNewDecFromInt(inactiveBabyAmount),
		Status:          stktypes.Unbonded, // Not bonded
	}
	require.NoError(t, stkKeeper.SetValidator(ctx, inactiveValidator))
	require.NoError(t, stkKeeper.SetDelegation(ctx, inactiveDelegation))
	// NOTE: Not calling SetLastValidatorPower - validator is NOT active

	// seed voting power dist cache with FP as active
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, btcDel.FpBtcPkList)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify co-staker was created with only the active validator's baby amount
	// Total baby should be 15000 (from active validator), not 25000 (15000 + 10000)
	verifyCoStakerCreated(t, ctx, cdc, storeService, stakerAddr, math.NewIntFromUint64(btcDel.TotalSat), activeBabyAmount)
}

func TestInitializeCoStakerRwdsTracker_MultipleCombinations(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Case 1: BTC + Baby staking (should create co-staker)
	staker1Addr := datagen.GenRandomAccount().GetAddress()
	btcDel1 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker1Addr, 60000)

	// Case 2: Only BTC staking (should create co-staker with 0 baby amt)
	staker2Addr := datagen.GenRandomAccount().GetAddress()
	btcDel2 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker2Addr, 80000)

	// Case 3: BTC to inactive FP + Baby staking (should create co-staker with 0 BTC amt)
	staker3Addr := datagen.GenRandomAccount().GetAddress()
	createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker3Addr, 100000)

	// create staking delegations - only staker1 and staker3 have baby staking
	babyDel1Amt := math.NewInt(30000)
	createBabyDelegation(t, ctx, stkKeeper, staker1Addr, babyDel1Amt)

	babyDel3Amt := math.NewInt(50000)
	createBabyDelegation(t, ctx, stkKeeper, staker3Addr, babyDel3Amt)

	// Collect all FP BTC public keys
	allFpBtcPks := make([]bbn.BIP340PubKey, 0)
	allFpBtcPks = append(allFpBtcPks, btcDel1.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel2.FpBtcPkList...)

	// seed voting power dist cache with all FPs as active
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, allFpBtcPks)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify results
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker1Addr, math.NewIntFromUint64(btcDel1.TotalSat), babyDel1Amt)    // Co-staker
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker2Addr, math.NewIntFromUint64(btcDel2.TotalSat), math.ZeroInt()) // BTC only
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker3Addr, math.ZeroInt(), babyDel3Amt)                             // FP not active

	// Verify total count
	count := countCoStakers(t, ctx, cdc, storeService)
	require.Equal(t, 3, count, "Should have exactly 3 co-stakers created")
}

func TestInitializeCoStakerRwdsTracker_WithMultipleActiveFPs(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create multiple test staker addresses
	staker1Addr := datagen.GenRandomAccount().GetAddress()
	staker2Addr := datagen.GenRandomAccount().GetAddress()
	staker3Addr := datagen.GenRandomAccount().GetAddress()

	// Create BTC delegations to different FPs
	btcDel1 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker1Addr, 30000)
	btcDel2 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker2Addr, 40000)
	btcDel3 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker3Addr, 50000)

	// Create baby staking delegations
	babyAmount1 := math.NewInt(15000)
	createBabyDelegation(t, ctx, stkKeeper, staker1Addr, babyAmount1)

	babyAmount2 := math.NewInt(20000)
	createBabyDelegation(t, ctx, stkKeeper, staker2Addr, babyAmount2)

	babyAmount3 := math.NewInt(25000)
	createBabyDelegation(t, ctx, stkKeeper, staker3Addr, babyAmount3)

	// Collect all FP BTC public keys
	allFpBtcPks := make([]bbn.BIP340PubKey, 0)
	allFpBtcPks = append(allFpBtcPks, btcDel1.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel2.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel3.FpBtcPkList...)

	// seed voting power dist cache with all FPs as active
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, allFpBtcPks)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify all co-stakers were created
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker1Addr, math.NewIntFromUint64(btcDel1.TotalSat), babyAmount1)
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker2Addr, math.NewIntFromUint64(btcDel2.TotalSat), babyAmount2)
	verifyCoStakerCreated(t, ctx, cdc, storeService, staker3Addr, math.NewIntFromUint64(btcDel3.TotalSat), babyAmount3)

	// Verify total count
	count := countCoStakers(t, ctx, cdc, storeService)
	require.Equal(t, 3, count, "Should have exactly 3 co-stakers created")
}

func TestInitializeCoStakerRwdsTracker_MultipleStakingFromSameStaker(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t)
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

	// Collect all FP BTC public keys
	allFpBtcPks := make([]bbn.BIP340PubKey, 0)
	allFpBtcPks = append(allFpBtcPks, btcDel1.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel2.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel3.FpBtcPkList...)

	// seed voting power dist cache with all FPs as active
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, allFpBtcPks)

	// Execute upgrade function
	err := v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
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

// setupVotingPowerDistCacheWithActiveFPs creates a voting power distribution cache
// with the specified FP BTC public keys as active finality providers.
// If the random cache doesn't have enough active FPs, it will replace inactive FPs
// with the provided ones and mark them as active.
func setupVotingPowerDistCacheWithActiveFPs(
	t *testing.T,
	r *rand.Rand,
	ctx sdk.Context,
	fKeeper *fkeeper.Keeper,
	fpBtcPks []bbn.BIP340PubKey,
) {
	// Generate random voting power dist cache
	vp, _, err := datagen.GenRandomVotingPowerDistCache(r, 10)
	require.NoError(t, err)
	require.NotEmpty(t, vp.FinalityProviders)

	activeFPsNeeded := len(fpBtcPks)

	// Ensure we have enough FPs in the cache
	for len(vp.FinalityProviders) < activeFPsNeeded {
		// Generate additional random FP
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		fpDistInfo := ftypes.NewFinalityProviderDistInfo(fp)
		fpDistInfo.TotalBondedSat = datagen.RandomInt(r, 10000) + 1000
		fpDistInfo.IsTimestamped = true
		vp.AddFinalityProviderDistInfo(fpDistInfo)
	}

	// Replace the first N FPs with the desired ones
	for i, fpBtcPk := range fpBtcPks {
		if i < len(vp.FinalityProviders) {
			// Keep the existing FP structure but replace the BTC public key
			vp.FinalityProviders[i].BtcPk = &fpBtcPk
			// Ensure this FP is timestamped and has bonded sats to be active
			vp.FinalityProviders[i].IsTimestamped = true
			if vp.FinalityProviders[i].TotalBondedSat == 0 {
				vp.FinalityProviders[i].TotalBondedSat = datagen.RandomInt(r, 10000) + 1000
			}
		}
	}

	// Apply active finality providers to ensure they are marked as active
	vp.ApplyActiveFinalityProviders(uint32(max(activeFPsNeeded, 10)))

	// Verify we have the expected active FPs
	activeFPs := vp.GetActiveFinalityProviderSet()
	require.True(t, len(activeFPs) >= activeFPsNeeded,
		"Expected at least %d active FPs, got %d", activeFPsNeeded, len(activeFPs))

	// Verify our desired FPs are among the active ones
	for _, fpBtcPk := range fpBtcPks {
		fpHex := fpBtcPk.MarshalHex()
		_, found := activeFPs[fpHex]
		require.True(t, found, "FP %s should be active", fpHex)
	}

	// Set the voting power distribution cache
	fKeeper.SetVotingPowerDistCache(ctx, uint64(ctx.HeaderInfo().Height)-1, vp)
}

func createTestBTCDelegation(t *testing.T, r *rand.Rand, ctx sdk.Context, btcStkKeeper btcstkkeeper.Keeper, stakerAddr sdk.AccAddress, stakingValue uint64) *btcstktypes.BTCDelegation {
	// Generate random BTC keys
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	// Generate finality provider
	fp, err := datagen.GenRandomFinalityProvider(r)
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

	// Create validator and mark it as active (bonded with power)
	validator := stktypes.Validator{
		OperatorAddress: validatorAddr.String(),
		Tokens:          delAmount,
		DelegatorShares: math.LegacyNewDecFromInt(delAmount),
		Status:          stktypes.Bonded,
	}
	require.NoError(t, stkKeeper.SetValidator(ctx, validator))
	require.NoError(t, stkKeeper.SetDelegation(ctx, delegation))

	// Add to LastValidatorPowers to mark as active validator
	power := stkKeeper.TokensToConsensusPower(ctx, delAmount)
	require.NoError(t, stkKeeper.SetLastValidatorPower(ctx, validatorAddr, power))
}

func verifyCoStakerCreated(t *testing.T, ctx sdk.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService, stakerAddr sdk.AccAddress, expectedBTCAmount, expectedBabyAmount math.Int) {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	tracker, err := rwdTrackers.Get(ctx, []byte(stakerAddr))

	require.NoError(t, err, "Co-staker rewards tracker should exist for %s", stakerAddr.String())
	require.Equal(t, uint64(1), tracker.StartPeriodCumulativeReward, "StartPeriodCumulativeReward should be 1")
	require.True(t, tracker.ActiveSatoshis.Equal(expectedBTCAmount), "ActiveSatoshis should match expected BTC amount: expected %s, got %s", expectedBTCAmount.String(), tracker.ActiveSatoshis.String())
	require.True(t, tracker.ActiveBaby.Equal(expectedBabyAmount), "ActiveBaby should match expected baby amount: expected %s, got %s", expectedBabyAmount.String(), tracker.ActiveBaby.String())
}

func countCoStakers(t *testing.T, ctx sdk.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService) int {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	var count int
	err := rwdTrackers.Walk(ctx, nil, func(key []byte, value costktypes.CostakerRewardsTracker) (stop bool, err error) {
		count++
		return false, nil
	})
	require.NoError(t, err)
	return count
}

func rwdTrackerCollection(storeService corestore.KVStoreService, cdc codec.BinaryCodec) collections.Map[[]byte, costktypes.CostakerRewardsTracker] {
	sb := collections.NewSchemaBuilder(storeService)
	rwdTrackers := collections.NewMap(
		sb,
		costktypes.CostakerRewardsTrackerKeyPrefix,
		"costaker_rewards_tracker",
		collections.BytesKey,
		codec.CollValue[costktypes.CostakerRewardsTracker](cdc),
	)
	return rwdTrackers
}


