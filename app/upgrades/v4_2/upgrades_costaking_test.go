package v4_2_test

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

	v4_2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_2"
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

func setupTestKeepers(t *testing.T, btcTip uint32) (sdk.Context, codec.BinaryCodec, corestore.KVStoreService, *stkkeeper.Keeper, btcstkkeeper.Keeper, *costkkeeper.Keeper, *fkeeper.Keeper, *gomock.Controller) {
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

	return btcCtx, cdc, costkStoreService, stkKeeper, *btcStkKeeper, costkKeeper, fKeeper, ctrl
}

// TestResetCoStakerRwdsTracker_WithPreexistingTrackers tests that existing trackers are reset
// and recalculated correctly with pre-existing costaker rewards trackers
func TestResetCoStakerRwdsTracker_WithPreexistingTrackers(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, costkKeeper.SetParams(ctx, costktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a test staker address
	stakerAddr := datagen.GenRandomAccount().GetAddress()

	// Create pre-existing costaker rewards tracker with arbitrary values
	preexistingTracker := costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: uint64(5),
		ActiveSatoshis:              math.NewInt(999999), // Wrong value
		ActiveBaby:                  math.NewInt(888888), // Wrong value
	}
	createCostakerRewardsTracker(t, ctx, cdc, storeService, stakerAddr, preexistingTracker)

	currPeriod := uint64(10)
	setCurrentRewardsPeriod(t, ctx, costkKeeper, currPeriod)

	// Create BTC delegation
	btcDel := createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr, 50000)

	// seed voting power dist cache with FP as active
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, btcDel.FpBtcPkList)

	// Execute reset function
	err := v4_2.ResetCoStakerRwdsTracker(
		ctx, cdc, storeService, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify tracker was reset and recalculated correctly
	verifyCoStakerUpdated(t, ctx, cdc, storeService, stakerAddr, math.NewIntFromUint64(btcDel.TotalSat), preexistingTracker.ActiveBaby, currPeriod)

	// Current rewards period should have increased
	currRwds, err := costkKeeper.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currPeriod+1, currRwds.Period, "Current rewards period should be updated")
}

// TestResetCoStakerRwdsTracker_MultiplePreexistingTrackers tests resetting multiple trackers
func TestResetCoStakerRwdsTracker_MultiplePreexistingTrackers(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, costkKeeper.SetParams(ctx, costktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create three stakers
	staker1Addr := datagen.GenRandomAccount().GetAddress()
	staker2Addr := datagen.GenRandomAccount().GetAddress()
	staker3Addr := datagen.GenRandomAccount().GetAddress()
	staker4Addr := datagen.GenRandomAccount().GetAddress()

	babyAmount1 := math.NewInt(15000)
	babyAmount2 := math.NewInt(20000)
	babyAmount3 := math.NewInt(25000)
	babyAmount4 := math.NewInt(0)

	// Create pre-existing trackers with incorrect values
	createCostakerRewardsTracker(t, ctx, cdc, storeService, staker1Addr, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: uint64(10),
		ActiveSatoshis:              math.NewInt(111111),
		ActiveBaby:                  babyAmount1,
	})
	createCostakerRewardsTracker(t, ctx, cdc, storeService, staker2Addr, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: uint64(15),
		ActiveSatoshis:              math.NewInt(333333),
		ActiveBaby:                  babyAmount2,
	})
	createCostakerRewardsTracker(t, ctx, cdc, storeService, staker3Addr, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: uint64(20),
		ActiveSatoshis:              math.NewInt(555555),
		ActiveBaby:                  babyAmount3,
	})

	startPeriod4 := uint64(25)
	activeSats4 := math.NewInt(75000)
	createCostakerRewardsTracker(t, ctx, cdc, storeService, staker4Addr, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: startPeriod4,
		ActiveSatoshis:              activeSats4,
		ActiveBaby:                  babyAmount4,
	})

	currPeriod := uint64(30)
	setCurrentRewardsPeriod(t, ctx, costkKeeper, currPeriod)

	// Create actual delegations for each staker
	btcDel1 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker1Addr, 30000)
	btcDel2 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker2Addr, 40000)
	btcDel3 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker3Addr, 50000)
	// del4 has multiple delegations
	btcDel41 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker4Addr, activeSats4.Uint64()/2)
	btcDel42 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker4Addr, activeSats4.Uint64()/2)

	// Collect all FP BTC public keys
	allFpBtcPks := make([]bbn.BIP340PubKey, 0)
	allFpBtcPks = append(allFpBtcPks, btcDel1.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel2.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel3.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel41.FpBtcPkList...)
	allFpBtcPks = append(allFpBtcPks, btcDel42.FpBtcPkList...)

	// seed voting power dist cache with all FPs as active
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, allFpBtcPks)

	// Execute reset function
	err := v4_2.ResetCoStakerRwdsTracker(
		ctx, cdc, storeService, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify all trackers were reset and recalculated correctly
	verifyCoStakerUpdated(t, ctx, cdc, storeService, staker1Addr, math.NewIntFromUint64(btcDel1.TotalSat), babyAmount1, currPeriod)
	verifyCoStakerUpdated(t, ctx, cdc, storeService, staker2Addr, math.NewIntFromUint64(btcDel2.TotalSat), babyAmount2, currPeriod)
	verifyCoStakerUpdated(t, ctx, cdc, storeService, staker3Addr, math.NewIntFromUint64(btcDel3.TotalSat), babyAmount3, currPeriod)

	// Active sats before == current active sats, so start period should not increase
	verifyCoStakerUpdated(t, ctx, cdc, storeService, staker4Addr, activeSats4, babyAmount4, startPeriod4)

	// Verify total count is still 4
	count := countCoStakers(t, ctx, cdc, storeService)
	require.Equal(t, 4, count, "Should have exactly 4 co-stakers")

	// Current rewards period should have increased
	currRwds, err := costkKeeper.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currPeriod+1, currRwds.Period, "Current rewards period should be updated")
}

// TestResetCoStakerRwdsTracker_TrackerNoLongerValid tests that trackers for stakers
// who no longer have delegations are zeroed out
func TestResetCoStakerRwdsTracker_TrackerNoLongerValid(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, costkKeeper.SetParams(ctx, costktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create two stakers
	staker1Addr := datagen.GenRandomAccount().GetAddress() // Will have delegations
	staker2Addr := datagen.GenRandomAccount().GetAddress() // Will NOT have delegations

	// Create pre-existing trackers for both
	babyAmount1 := math.NewInt(15000)
	babyAmount2 := math.NewInt(100000)

	startPeriod1 := uint64(5)
	createCostakerRewardsTracker(t, ctx, cdc, storeService, staker1Addr, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: startPeriod1,
		ActiveSatoshis:              math.NewInt(100000),
		ActiveBaby:                  babyAmount1,
	})
	startPeriod2 := uint64(10)
	createCostakerRewardsTracker(t, ctx, cdc, storeService, staker2Addr, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: startPeriod2,
		ActiveSatoshis:              math.NewInt(200000),
		ActiveBaby:                  babyAmount2,
	})

	currPeriod := uint64(20)
	setCurrentRewardsPeriod(t, ctx, costkKeeper, currPeriod)

	// Create delegations ONLY for staker1
	btcDel1 := createTestBTCDelegation(t, r, ctx, btcStkKeeper, staker1Addr, 30000)

	// Setup voting power dist cache
	setupVotingPowerDistCacheWithActiveFPs(t, r, ctx, fKeeper, btcDel1.FpBtcPkList)

	// Execute reset function
	err := v4_2.ResetCoStakerRwdsTracker(
		ctx, cdc, storeService, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify staker1 has correct values
	verifyCoStakerUpdated(t, ctx, cdc, storeService, staker1Addr, math.NewIntFromUint64(btcDel1.TotalSat), babyAmount1, currPeriod)

	// Verify staker2 tracker is zeroed out (no delegations)
	verifyCoStakerUpdated(t, ctx, cdc, storeService, staker2Addr, math.ZeroInt(), babyAmount2, currPeriod)

	// Current rewards period should have increased
	currRwds, err := costkKeeper.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currPeriod+1, currRwds.Period, "Current rewards period should be updated")
}

// TestResetCoStakerRwdsTracker_InactiveFPAndValidator tests resetting when FP or validator becomes inactive
func TestResetCoStakerRwdsTracker_InactiveFPAndValidator(t *testing.T) {
	ctx, cdc, storeService, stkKeeper, btcStkKeeper, costkKeeper, fKeeper, ctrl := setupTestKeepers(t, 10)
	defer ctrl.Finish()

	require.NoError(t, btcStkKeeper.SetParams(ctx, btcstktypes.DefaultParams()))
	require.NoError(t, costkKeeper.SetParams(ctx, costktypes.DefaultParams()))
	require.NoError(t, stkKeeper.SetParams(ctx, stktypes.DefaultParams()))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	stakerAddr1 := datagen.GenRandomAccount().GetAddress()
	stakerAddr2 := datagen.GenRandomAccount().GetAddress()

	// Create pre-existing tracker with both BTC and BABY
	babyAmount := math.NewInt(25000)
	startPeriod := uint64(5)
	createCostakerRewardsTracker(t, ctx, cdc, storeService, stakerAddr1, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: startPeriod,
		ActiveSatoshis:              math.NewInt(100000),
		ActiveBaby:                  babyAmount,
	})

	// staker 2 has costaker tracker with 0 sats and some baby
	createCostakerRewardsTracker(t, ctx, cdc, storeService, stakerAddr2, costktypes.CostakerRewardsTracker{
		StartPeriodCumulativeReward: startPeriod,
		ActiveSatoshis:              math.ZeroInt(),
		ActiveBaby:                  babyAmount,
	})

	// Create BTC delegation (but FP will be inactive)
	createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr1, 50000)
	createTestBTCDelegation(t, r, ctx, btcStkKeeper, stakerAddr2, 50000)

	currPeriod := uint64(20)
	setCurrentRewardsPeriod(t, ctx, costkKeeper, currPeriod)

	// Setup voting power dist cache WITHOUT the FP (making it inactive)
	vp, _, err := datagen.GenRandomVotingPowerDistCache(r, 10)
	require.NoError(t, err)
	fKeeper.SetVotingPowerDistCache(ctx, uint64(ctx.HeaderInfo().Height)-1, vp)

	// Execute reset function
	err = v4_2.ResetCoStakerRwdsTracker(
		ctx, cdc, storeService, btcStkKeeper, *costkKeeper, *fKeeper,
	)
	require.NoError(t, err)

	// Verify tracker is zeroed out (active sats only)
	verifyCoStakerUpdated(t, ctx, cdc, storeService, stakerAddr1, math.ZeroInt(), babyAmount, currPeriod)
	// For staker2, there's no change as it had 0 sats before
	// so start period should remain unchanged
	verifyCoStakerUpdated(t, ctx, cdc, storeService, stakerAddr2, math.ZeroInt(), babyAmount, startPeriod)

	// Current rewards period should have increased
	currRwds, err := costkKeeper.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currPeriod+1, currRwds.Period, "Current rewards period should be updated")
}

// Helper functions

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
			vp.FinalityProviders[i].BtcPk = &fpBtcPk
			vp.FinalityProviders[i].IsTimestamped = true
			if vp.FinalityProviders[i].TotalBondedSat == 0 {
				vp.FinalityProviders[i].TotalBondedSat = datagen.RandomInt(r, 10000) + 1000
			}
		}
	}

	// Apply active finality providers
	vp.ApplyActiveFinalityProviders(uint32(max(activeFPsNeeded, 10)))

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

func createCostakerRewardsTracker(t *testing.T, ctx context.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService, stakerAddr sdk.AccAddress, tracker costktypes.CostakerRewardsTracker) {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	err := rwdTrackers.Set(ctx, []byte(stakerAddr), tracker)
	require.NoError(t, err)
}

func verifyCoStakerUpdated(t *testing.T, ctx sdk.Context, cdc codec.BinaryCodec, storeService corestore.KVStoreService, stakerAddr sdk.AccAddress, expectedBTCAmount, expectedBabyAmount math.Int, expectedStartPeriod uint64) {
	rwdTrackers := rwdTrackerCollection(storeService, cdc)
	tracker, err := rwdTrackers.Get(ctx, []byte(stakerAddr))

	require.NoError(t, err, "Co-staker rewards tracker should exist for %s", stakerAddr.String())
	require.Equal(t, expectedStartPeriod, tracker.StartPeriodCumulativeReward, "StartPeriodCumulativeReward should be %d", expectedStartPeriod)
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

func setCurrentRewardsPeriod(t *testing.T, ctx sdk.Context, costkKeeper *costkkeeper.Keeper, period uint64) {
	_, err := costkKeeper.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)
	endedPeriod := uint64(0)
	for endedPeriod < uint64(period-1) {
		endedPeriod, err = costkKeeper.IncrementRewardsPeriod(ctx)
		require.NoError(t, err)
	}

	currRwds, err := costkKeeper.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, period, currRwds.Period, "current period is %d", currRwds.Period)
}
