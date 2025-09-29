package v4_test

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/collections"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	"cosmossdk.io/store/prefix"
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

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
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

const (
	mainnet                      = "mainnet"
	testDataDir                  = "testdata"
	mainnetBabyDelegationsFile   = "mainnet-baby-delegations.json"
	testnetBabyDelegationsFile   = "testnet-baby-delegations.json"
	btcDelegationsFile           = "btc-delegations.json.test" // Note: ".test" suffix to avoid accidental git add of large file
	mainnetCostakerAddressesFile = "mainnet-costaker-addresses.txt"
	testnetCostakerAddressesFile = "testnet-costaker-addresses.txt"
)

func setupTestKeepers(t *testing.T, btcTip uint32) (sdk.Context, codec.BinaryCodec, corestore.KVStoreService, *stkkeeper.Keeper, btcstkkeeper.Keeper, *storetypes.KVStoreKey, *costkkeeper.Keeper, *fkeeper.Keeper, *gomock.Controller) {
	ctrl := gomock.NewController(t)

	// Create DB and store
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	// Setup mocked keepers
	btclcKeeper := btcstktypes.NewMockBTCLightClientKeeper(ctrl)
	btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: btcTip}).AnyTimes()

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

	return btcCtx, cdc, costkStoreService, stkKeeper, *btcStkKeeper, btcStkStoreKey, costkKeeper, fKeeper, ctrl
}

func TestInitializeCoStakerRwdsTracker_EmptyState(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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

func TestInitializeCoStakerRwdsTracker_WithRealDelegations(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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

func TestInitializeCoStakerRwdsTracker_MultipleCombinations(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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
	require.Equal(t, 3, count, "Should have exactly 3 co-staker created")
}

func TestInitializeCoStakerRwdsTracker_WithMultipleActiveFPs(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, _, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
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

func TestInitializeCoStakerRwdsTracker_TestnetData(t *testing.T) {
	runTestWithEnv(t, "testnet", 268000)
}

func TestInitializeCoStakerRwdsTracker_MainnetData(t *testing.T) {
	runTestWithEnv(t, mainnet, 914000)
}

func runTestWithEnv(t *testing.T, env string, btcTip uint32) {
	require.True(t, env == mainnet || env == "testnet", "env must be 'mainnet' or 'testnet'")
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, btcStkKey, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, btcTip)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	t.Log("Loading testnet data...")

	// Load expected costaker addresses first (small file)
	expectedCostakers, err := loadCostakers(env)
	require.NoError(t, err)
	require.NotEmpty(t, expectedCostakers)
	t.Logf("Expected %d costakers from %s data", len(expectedCostakers), env)

	// Load and seed BTC delegations using streaming
	btcDelCount, fpPubKeys, err := loadAndSeedBTCDelegations(t, ctx, env, btcStkKey)
	require.NoError(t, err)
	t.Logf("Loaded and seeded %d BTC delegations", btcDelCount)
	t.Logf("Found %d unique finality providers", len(fpPubKeys))

	// Load and seed cosmos delegations using streaming
	cosmosDelCount, err := loadAndSeedCosmosDelegations(t, ctx, env, stkKeeper)
	require.NoError(t, err)
	t.Logf("Loaded and seeded %d cosmos delegations", cosmosDelCount)

	// Setup voting power distribution cache with active FPs from the loaded data
	t.Log("Setting up voting power distribution cache with active FPs...")
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, fpPubKeys)

	t.Log("Executing upgrade function...")

	// Execute upgrade function
	err = v4.InitializeCoStakerRwdsTracker(
		ctx, cdc, storeService, stkKeeper, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify costakers were created
	actualCostakers := getAllCostakers(t, ctx, cdc, storeService)
	t.Logf("Created %d costakers", len(actualCostakers))

	// Verify that created costakers match expected testnet costakers
	expectedSet := make(map[string]bool)
	for _, addr := range expectedCostakers {
		expectedSet[addr] = true
	}

	actualSet := make(map[string]bool)
	for addr := range actualCostakers {
		actualSet[addr] = true
	}

	// Check that all expected costakers were created
	missingCostakers := 0
	for expectedAddr := range expectedSet {
		if !actualSet[expectedAddr] {
			t.Errorf("Expected costaker %s was not created", expectedAddr)
			missingCostakers++
		}
	}

	require.Equal(t, 0, missingCostakers, "Found %d missing costakers", missingCostakers)

	// Verify tracker content: expected costakers should have both activeSat > 0 AND activeBaby > 0
	// All other costakers should have either activeSat = 0 OR activeBaby = 0
	expectedCostakersWithBothActive := 0
	otherCostakersWithBothActive := 0

	for addr, tracker := range actualCostakers {
		hasBothActive := tracker.ActiveSatoshis.GT(math.ZeroInt()) && tracker.ActiveBaby.GT(math.ZeroInt())

		if expectedSet[addr] {
			// This is an expected costaker - should have both BTC and BABY > 0
			if !hasBothActive {
				t.Errorf("Expected costaker %s should have both activeSat > 0 AND activeBaby > 0, but got activeSat=%s, activeBaby=%s",
					addr, tracker.ActiveSatoshis.String(), tracker.ActiveBaby.String())
			} else {
				expectedCostakersWithBothActive++
			}
		} else {
			// This costaker was not in the expected list - should have either activeSat = 0 OR activeBaby = 0
			if hasBothActive {
				t.Errorf("Non-expected costaker %s should have either activeSat = 0 OR activeBaby = 0, but got both > 0: activeSat=%s, activeBaby=%s",
					addr, tracker.ActiveSatoshis.String(), tracker.ActiveBaby.String())
				otherCostakersWithBothActive++
			}
		}
	}

	require.Equal(t, len(expectedCostakers), expectedCostakersWithBothActive,
		"Number of created costakers (%d) should match expected (%d)",
		expectedCostakersWithBothActive, len(expectedCostakers))

	t.Logf("Verification complete: %d expected costakers have both BTC and BABY staking active", expectedCostakersWithBothActive)
	t.Logf("All %d %s costakers were created correctly with proper staking amounts", len(actualCostakers), env)
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

// convertBTCDelegationResponseToBTCDelegation converts a BTCDelegationResponse to BTCDelegation
func convertBTCDelegationResponseToBTCDelegation(resp *btcstktypes.BTCDelegationResponse) (*btcstktypes.BTCDelegation, error) {
	// Decode hex strings to bytes
	stakingTx, err := hex.DecodeString(resp.StakingTxHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode staking tx hex: %w", err)
	}

	var slashingTx []byte
	if resp.SlashingTxHex != "" {
		slashingTx, err = hex.DecodeString(resp.SlashingTxHex)
		if err != nil {
			return nil, fmt.Errorf("failed to decode slashing tx hex: %w", err)
		}
	}

	// Decode delegator signature
	delegatorSig, err := bbn.NewBIP340SignatureFromHex(resp.DelegatorSlashSigHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator signature: %w", err)
	}

	del := &btcstktypes.BTCDelegation{
		StakerAddr:       resp.StakerAddr,
		BtcPk:            resp.BtcPk,
		Pop:              nil, // This will need to be handled separately or left nil for migration
		FpBtcPkList:      resp.FpBtcPkList,
		StakingTime:      resp.StakingTime,
		StartHeight:      resp.StartHeight,
		EndHeight:        resp.EndHeight,
		TotalSat:         resp.TotalSat,
		StakingTx:        stakingTx,
		StakingOutputIdx: resp.StakingOutputIdx,
		DelegatorSig:     delegatorSig,
		CovenantSigs:     resp.CovenantSigs,
		UnbondingTime:    resp.UnbondingTime,
		ParamsVersion:    resp.ParamsVersion,
		BtcUndelegation:  nil, // Initialize as nil, will be set if undelegation data exists
	}

	if slashingTx != nil {
		del.SlashingTx = btcstktypes.NewBtcSlashingTxFromBytes(slashingTx)
	}

	// Handle undelegation response if it exists
	if resp.UndelegationResponse != nil {
		// The UndelegationResponse should have unbonding_tx_hex field
		unbondingTx := make([]byte, 0)
		if resp.UndelegationResponse.UnbondingTxHex != "" {
			unbondingTx, err = hex.DecodeString(resp.UndelegationResponse.UnbondingTxHex)
			if err != nil {
				return nil, fmt.Errorf("failed to decode unbonding tx hex: %w", err)
			}
		}
		del.BtcUndelegation = &btcstktypes.BTCUndelegation{
			UnbondingTx:              unbondingTx,
			CovenantUnbondingSigList: resp.UndelegationResponse.CovenantUnbondingSigList,
			CovenantSlashingSigs:     resp.UndelegationResponse.CovenantSlashingSigs,
		}
	}

	return del, nil
}

// loadAndSeedBTCDelegations loads BTC delegations from file and seeds them into keeper using streaming
// It also collects and returns unique finality provider BTC public keys found in the delegations
func loadAndSeedBTCDelegations(t *testing.T, ctx sdk.Context, env string, btcStkStoreKey *storetypes.KVStoreKey) (int, []bbn.BIP340PubKey, error) {
	filePath := filepath.Join(testDataDir, btcDelegationsFile)

	// Check if file exists. Should be downloaded or got from cache by CI workflow
	// if not (running locally e.g.) download from Google Drive
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Logf("File %s does not exist, downloading from Google Drive...", filePath)
		if err := downloadBTCDelegationsFile(filePath); err != nil {
			return 0, nil, fmt.Errorf("failed to download BTC delegations file: %w", err)
		}
		t.Logf("Successfully downloaded %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to open BTC delegations file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// Read opening brace for the wrapper object
	token, err := decoder.Token()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read opening brace: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return 0, nil, fmt.Errorf("expected opening brace, got %v", token)
	}

	// Read through the JSON keys to find the correct key ("testnet" or "mainnet")
	var foundEnvKey bool
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read key: %w", err)
		}

		if key, ok := token.(string); ok {
			if key == env {
				foundEnvKey = true
				break
			} else {
				// Skip the value for this key (the array we don't want)
				var dummy json.RawMessage
				if err := decoder.Decode(&dummy); err != nil {
					return 0, nil, fmt.Errorf("failed to skip %s data: %w", key, err)
				}
			}
		}
	}

	if !foundEnvKey {
		return 0, nil, fmt.Errorf("could not find %s key in JSON", env)
	}

	// Read opening bracket for the array
	token, err = decoder.Token()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read opening bracket: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return 0, nil, fmt.Errorf("expected opening bracket, got %v", token)
	}

	codec := appparams.DefaultEncodingConfig().Codec
	storeService := runtime.NewKVStoreService(btcStkStoreKey)
	storeAdapter := runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, btcstktypes.BTCDelegationKey)

	// Track unique finality provider public keys
	fpPubKeyMap := make(map[string]bbn.BIP340PubKey)
	count := 0
	for decoder.More() {
		var delResp btcstktypes.BTCDelegationResponse
		if err := decoder.Decode(&delResp); err != nil {
			return 0, nil, fmt.Errorf("failed to decode BTC delegation %d: %w", count, err)
		}

		// Convert BTCDelegationResponse to BTCDelegation
		del, err := convertBTCDelegationResponseToBTCDelegation(&delResp)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to convert BTC delegation %d: %w", count, err)
		}

		del.ParamsVersion = 0

		// Collect unique FP public keys
		for _, fpPubKey := range del.FpBtcPkList {
			fpHex := fpPubKey.MarshalHex()
			fpPubKeyMap[fpHex] = fpPubKey
		}

		stakingTxHash := del.MustGetStakingTxHash()
		btcDelBytes := codec.MustMarshal(del)
		store.Set(stakingTxHash[:], btcDelBytes)

		count++
		if count%10000 == 0 {
			t.Logf("Processed %d BTC delegations...", count)
		}
	}

	// Read closing bracket for the array
	token, err = decoder.Token()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read closing bracket: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != ']' {
		return 0, nil, fmt.Errorf("expected closing bracket, got %v", token)
	}

	// Skip any remaining keys in the JSON object (e.g., if we processed mainnet but testnet is still there)
	for decoder.More() {
		// Read key
		_, err = decoder.Token()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read remaining key: %w", err)
		}
		// Skip the value for this key
		var dummy json.RawMessage
		if err := decoder.Decode(&dummy); err != nil {
			return 0, nil, fmt.Errorf("failed to skip remaining data: %w", err)
		}
	}

	// Read closing brace for the wrapper object
	token, err = decoder.Token()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read closing brace: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '}' {
		return 0, nil, fmt.Errorf("expected closing brace, got %v", token)
	}

	// Convert map to slice for return
	fpPubKeys := make([]bbn.BIP340PubKey, 0, len(fpPubKeyMap))
	for _, fpPubKey := range fpPubKeyMap {
		fpPubKeys = append(fpPubKeys, fpPubKey)
	}

	return count, fpPubKeys, nil
}

// loadAndSeedCosmosDelegations loads cosmos delegations from file and seeds them into keeper using streaming
func loadAndSeedCosmosDelegations(t *testing.T, ctx sdk.Context, env string, stkKeeper *stkkeeper.Keeper) (int, error) {
	fileName := testnetBabyDelegationsFile
	if env == mainnet {
		fileName = mainnetBabyDelegationsFile
	}
	filePath := filepath.Join(testDataDir, fileName)

	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open cosmos delegations file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// Read opening bracket
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("failed to read opening bracket: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return 0, fmt.Errorf("expected opening bracket, got %v", token)
	}

	validators := make(map[string]bool)
	count := 0

	for decoder.More() {
		var rawDel struct {
			Delegation struct {
				DelegatorAddress string `json:"delegator_address"`
				ValidatorAddress string `json:"validator_address"`
				Shares           string `json:"shares"`
			} `json:"delegation"`
		}

		if err := decoder.Decode(&rawDel); err != nil {
			return 0, fmt.Errorf("failed to decode cosmos delegation %d: %w", count, err)
		}

		shares, err := math.LegacyNewDecFromStr(rawDel.Delegation.Shares)
		if err != nil {
			return 0, fmt.Errorf("failed to parse shares %s for delegation %d: %w", rawDel.Delegation.Shares, count, err)
		}

		// Create validator if not exists
		if !validators[rawDel.Delegation.ValidatorAddress] {
			validator := stktypes.Validator{
				OperatorAddress: rawDel.Delegation.ValidatorAddress,
				Tokens:          math.ZeroInt(),
				DelegatorShares: math.LegacyZeroDec(),
			}
			if err := stkKeeper.SetValidator(ctx, validator); err != nil {
				return 0, fmt.Errorf("failed to set validator %s: %w", rawDel.Delegation.ValidatorAddress, err)
			}
			validators[rawDel.Delegation.ValidatorAddress] = true
		}

		// Add delegation
		delegation := stktypes.Delegation{
			DelegatorAddress: rawDel.Delegation.DelegatorAddress,
			ValidatorAddress: rawDel.Delegation.ValidatorAddress,
			Shares:           shares,
		}

		if err := stkKeeper.SetDelegation(ctx, delegation); err != nil {
			return 0, fmt.Errorf("failed to set delegation %d: %w", count, err)
		}

		// Update validator shares and tokens
		validator, err := stkKeeper.GetValidator(ctx, sdk.MustValAddressFromBech32(rawDel.Delegation.ValidatorAddress))
		if err != nil {
			return 0, fmt.Errorf("validator %s not found after creation", rawDel.Delegation.ValidatorAddress)
		}
		validator.Tokens = validator.Tokens.Add(shares.TruncateInt())
		validator.DelegatorShares = validator.DelegatorShares.Add(shares)
		if err := stkKeeper.SetValidator(ctx, validator); err != nil {
			return 0, fmt.Errorf("failed to update validator %s: %w", rawDel.Delegation.ValidatorAddress, err)
		}

		count++
		if count%1000 == 0 {
			t.Logf("Processed %d cosmos delegations...", count)
		}
	}

	// Read closing bracket
	token, err = decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("failed to read closing bracket: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != ']' {
		return 0, fmt.Errorf("expected closing bracket, got %v", token)
	}

	return count, nil
}

// loadCostakers loads expected costaker addresses for provided env (testnet/mainnet)
func loadCostakers(env string) ([]string, error) {
	fileName := testnetCostakerAddressesFile
	if env == mainnet {
		fileName = mainnetCostakerAddressesFile
	}
	filePath := filepath.Join(testDataDir, fileName)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open costaker addresses file: %w", err)
	}
	defer file.Close()

	var addresses []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		addr := strings.TrimSpace(scanner.Text())
		if addr != "" {
			addresses = append(addresses, addr)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read costaker addresses: %w", err)
	}

	return addresses, nil
}

// getAllCostakers returns all costaker addresses created during the test
func getAllCostakers(t *testing.T, ctx sdk.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService) map[string]costktypes.CostakerRewardsTracker {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	costakers := make(map[string]costktypes.CostakerRewardsTracker)

	err := rwdTrackers.Walk(ctx, nil, func(key []byte, value costktypes.CostakerRewardsTracker) (stop bool, err error) {
		addr := sdk.AccAddress(key).String()
		costakers[addr] = value
		return false, nil
	})
	require.NoError(t, err)

	return costakers
}

// downloadBTCDelegationsFile downloads a file from Google Drive using the file ID
// This is useful when running tests locally and the test data files are not present
func downloadBTCDelegationsFile(filePath string) error {
	// Use the direct download URL that bypasses the virus scan warning for large files
	client := &http.Client{}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://drive.usercontent.google.com/download?id=1PaZe96acfJqCHJrc24VAh77H-z0U9_x1&export=download&confirm=t", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add user agent to avoid potential blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; babylon-test)")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file from Google Drive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: HTTP status %d", resp.StatusCode)
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
