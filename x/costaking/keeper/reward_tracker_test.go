package keeper

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/coins"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func FuzzAddRewardsForCostakers(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockIctvK := types.NewMockIncentiveKeeper(ctrl)
		k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

		rewards := datagen.GenRandomCoins(r)

		err := k.AddRewardsForCostakers(ctx, rewards)
		require.NoError(t, err)

		currentRwd, err := k.GetCurrentRewards(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(1), currentRwd.Period)
		require.Equal(t, rewards.MulInt(ictvtypes.DecimalRewards).String(), currentRwd.Rewards.String())
		require.Equal(t, sdkmath.ZeroInt().String(), currentRwd.TotalScore.String())

		additionalRewards := datagen.GenRandomCoins(r)
		err = k.AddRewardsForCostakers(ctx, additionalRewards)
		require.NoError(t, err)

		updatedCurrentRwd, err := k.GetCurrentRewards(ctx)
		require.NoError(t, err)
		expectedTotalRewards := rewards.Add(additionalRewards...).MulInt(ictvtypes.DecimalRewards)
		require.Equal(t, expectedTotalRewards.String(), updatedCurrentRwd.Rewards.String())
	})
}

func FuzzIncrementRewardsPeriod(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockIctvK := types.NewMockIncentiveKeeper(ctrl)
		k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

		endedPeriod, err := k.IncrementRewardsPeriod(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(1), endedPeriod)

		rewards := datagen.GenRandomCoins(r)
		totalScore := datagen.RandomMathInt(r, 1000000)

		err = k.AddRewardsForCostakers(ctx, rewards)
		require.NoError(t, err)

		err = k.UpdateCurrentRewardsTotalScore(ctx, totalScore)
		require.NoError(t, err)

		initialCurrentRwd, err := k.GetCurrentRewards(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(2), initialCurrentRwd.Period)
		require.Equal(t, totalScore.String(), initialCurrentRwd.TotalScore.String())

		endedPeriod, err = k.IncrementRewardsPeriod(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(2), endedPeriod)

		// After increment, period should advance to 3 and create historical rewards for period 2
		newCurrentRwd, err := k.GetCurrentRewards(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(3), newCurrentRwd.Period)
		require.True(t, newCurrentRwd.Rewards.IsZero())
		require.Equal(t, totalScore.String(), newCurrentRwd.TotalScore.String())

		// Historical rewards should be created for period 2
		historicalRwd, err := k.GetHistoricalRewards(ctx, 2)
		require.NoError(t, err)
		expectedRewardsPerScore := initialCurrentRwd.Rewards.QuoInt(totalScore)
		require.Equal(t, expectedRewardsPerScore.String(), historicalRwd.CumulativeRewardsPerScore.String())
	})
}

func FuzzCalculateCostakerRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

		costaker := datagen.GenRandomAddress()
		costakrScore := datagen.RandomMathInt(r, 10000).AddRaw(10)

		hist1 := datagen.GenRandomHistoricalRewards(r)
		hist1.CumulativeRewardsPerScore = hist1.CumulativeRewardsPerScore.MulInt(ictvtypes.DecimalRewards)
		startPeriod := datagen.RandomInt(r, 10)

		err := k.setHistoricalRewards(ctx, startPeriod, hist1)
		require.NoError(t, err)

		hist2 := types.NewHistoricalRewards(hist1.CumulativeRewardsPerScore.MulInt(sdkmath.NewInt(2)))
		endPeriod := startPeriod + 1 + datagen.RandomInt(r, 10)
		err = k.setHistoricalRewards(ctx, endPeriod, hist2)
		require.NoError(t, err)

		initialTracker := types.NewCostakerRewardsTrackerBasic(startPeriod, costakrScore)
		err = k.setCostakerRewardsTracker(ctx, costaker, initialTracker)
		require.NoError(t, err)

		expRwds := hist2.CumulativeRewardsPerScore.Sub(hist1.CumulativeRewardsPerScore...).MulInt(costakrScore).QuoInt(ictvtypes.DecimalRewards)
		rewards, err := k.CalculateCostakerRewards(ctx, costaker, endPeriod)
		require.NoError(t, err)
		require.Equal(t, expRwds.String(), rewards.String())
	})
}

func FuzzCalculateCostakerRewardsAndSendToGauge(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ictvK := types.NewMockIncentiveKeeper(ctrl)
		k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)

		costaker := datagen.GenRandomAddress()
		costakrScore := datagen.RandomMathInt(r, 10000).AddRaw(10)

		hist1 := datagen.GenRandomHistoricalRewards(r)
		hist1.CumulativeRewardsPerScore = hist1.CumulativeRewardsPerScore.MulInt(ictvtypes.DecimalRewards)
		startPeriod := datagen.RandomInt(r, 10)

		err := k.setHistoricalRewards(ctx, startPeriod, hist1)
		require.NoError(t, err)

		hist2 := types.NewHistoricalRewards(hist1.CumulativeRewardsPerScore.MulInt(sdkmath.NewInt(2)))
		endPeriod := startPeriod + 1 + datagen.RandomInt(r, 10)
		err = k.setHistoricalRewards(ctx, endPeriod, hist2)
		require.NoError(t, err)

		initialTracker := types.NewCostakerRewardsTrackerBasic(startPeriod, costakrScore)
		err = k.setCostakerRewardsTracker(ctx, costaker, initialTracker)
		require.NoError(t, err)

		expRwds := hist2.CumulativeRewardsPerScore.Sub(hist1.CumulativeRewardsPerScore...).MulInt(costakrScore).QuoInt(ictvtypes.DecimalRewards)

		ictvK.EXPECT().AccumulateRewardGaugeForCostaker(
			gomock.Any(),
			gomock.Eq(costaker),
			gomock.Eq(expRwds),
		).Times(1)

		bankK := k.bankK.(*types.MockBankKeeper)
		bankK.EXPECT().SendCoinsFromModuleToModule(
			gomock.Any(),
			gomock.Eq(types.ModuleName),
			gomock.Eq(ictvtypes.ModuleName),
			gomock.Eq(expRwds),
		).Return(nil).Times(1)

		err = k.CalculateCostakerRewardsAndSendToGauge(ctx, costaker, endPeriod)
		require.NoError(t, err)
	})
}

func FuzzGetCurrentRewardsInitialized(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockIctvK := types.NewMockIncentiveKeeper(ctrl)
		k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

		currentRwd, err := k.GetCurrentRewardsInitialized(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(1), currentRwd.Period)
		require.True(t, currentRwd.Rewards.IsZero())
		require.True(t, currentRwd.TotalScore.IsZero())

		historicalRwd, err := k.GetHistoricalRewards(ctx, 0)
		require.NoError(t, err)
		require.True(t, historicalRwd.CumulativeRewardsPerScore.IsZero())

		currentRwd2, err := k.GetCurrentRewardsInitialized(ctx)
		require.NoError(t, err)
		require.Equal(t, currentRwd.Period, currentRwd2.Period)
		require.Equal(t, currentRwd.Rewards.String(), currentRwd2.Rewards.String())
		require.Equal(t, currentRwd.TotalScore.String(), currentRwd2.TotalScore.String())
	})
}

func TestCalculateCostakerRewardsBetweenNegativeRewards(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

	r := rand.New(rand.NewSource(42))
	startingRewards := datagen.GenRandomCoins(r)
	endingRewards := sdk.NewCoins()

	startPeriod := uint64(1)
	endPeriod := startPeriod + 1
	err := k.setHistoricalRewards(ctx, startPeriod, types.NewHistoricalRewards(startingRewards))
	require.NoError(t, err)
	err = k.setHistoricalRewards(ctx, endPeriod, types.NewHistoricalRewards(endingRewards))
	require.NoError(t, err)

	tracker := types.NewCostakerRewardsTrackerBasic(startPeriod, sdkmath.NewInt(100))

	delta, _ := endingRewards.SafeSub(startingRewards...)
	_, err = k.calculateCoStakerRewardsBetween(ctx, tracker, endPeriod)
	require.EqualError(t, err, types.ErrNegativeRewards.Wrapf("cumulative rewards is negative %s", delta.String()).Error())
}

func TestCalculateCostakerRewardsBetweenInvalidPeriod(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

	startPeriod := uint64(10)
	tracker := types.NewCostakerRewardsTrackerBasic(startPeriod, sdkmath.NewInt(100))

	endPeriod := uint64(5)
	_, err := k.calculateCoStakerRewardsBetween(ctx, tracker, endPeriod)
	require.EqualError(t, err, types.ErrInvalidPeriod.Wrapf("startingPeriod %d cannot be greater than endingPeriod %d", startPeriod, endPeriod).Error())
}

func TestInitializeCostakerRwdTracker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	costaker := datagen.GenRandomAddress()
	totalScore := sdkmath.NewInt(1000)

	initialTracker := types.NewCostakerRewardsTrackerBasic(0, totalScore)
	err := k.setCostakerRewardsTracker(ctx, costaker, initialTracker)
	require.NoError(t, err)

	r := rand.New(rand.NewSource(42))
	err = k.AddRewardsForCostakers(ctx, datagen.GenRandomCoins(r))
	require.NoError(t, err)

	endedPeriod, err := k.IncrementRewardsPeriod(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), endedPeriod)

	err = k.initializeCoStakerRwdTracker(ctx, costaker)
	require.NoError(t, err)

	updatedTracker, err := k.GetCostakerRewards(ctx, costaker)
	require.NoError(t, err)

	currentRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	expectedStartPeriod := currentRwd.Period - 1

	require.Equal(t, expectedStartPeriod, updatedTracker.StartPeriodCumulativeReward)
	require.Equal(t, totalScore.String(), updatedTracker.TotalScore.String())
}

func TestCostakerRewardsFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ictvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)

	costakr1 := datagen.GenRandomAddress()
	costakr2 := datagen.GenRandomAddress()

	costakr1Score := sdkmath.NewInt(300)
	costakr2Score := sdkmath.NewInt(700)
	totalScore := costakr1Score.Add(costakr2Score)

	// Test the full flow by manually setting up historical rewards like the working tests do
	// Initialize the rewards tracking system first
	_, err := k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	tracker1 := types.NewCostakerRewardsTrackerBasic(0, costakr1Score)
	tracker2 := types.NewCostakerRewardsTrackerBasic(0, costakr2Score)

	err = k.setCostakerRewardsTracker(ctx, costakr1, tracker1)
	require.NoError(t, err)
	err = k.setCostakerRewardsTracker(ctx, costakr2, tracker2)
	require.NoError(t, err)

	// Manually create historical rewards for period 1 to test reward calculation
	// This simulates what would happen after a proper period transition
	endedPeriod := uint64(1)

	rewards := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100000)))
	rewardsPerScore := rewards.MulInt(ictvtypes.DecimalRewards).QuoInt(totalScore)
	historical1 := types.NewHistoricalRewards(rewardsPerScore)
	err = k.setHistoricalRewards(ctx, endedPeriod, historical1)
	require.NoError(t, err)

	// Calculate rewards for the manually created period
	rwd1, err := k.CalculateCostakerRewards(ctx, costakr1, endedPeriod)
	require.NoError(t, err)

	rwd2, err := k.CalculateCostakerRewards(ctx, costakr2, endedPeriod)
	require.NoError(t, err)

	// Expected rewards should be proportional to their scores
	expRwdCostaker1 := rewards.MulInt(costakr1Score).QuoInt(totalScore)
	expRwdCostaker2 := rewards.MulInt(costakr2Score).QuoInt(totalScore)

	require.Equal(t, expRwdCostaker1.String(), rwd1.String())
	require.Equal(t, expRwdCostaker2.String(), rwd2.String())

	// Mock expectations for both incentive gauge accumulation and bank transfer
	ictvK.EXPECT().AccumulateRewardGaugeForCostaker(ctx, costakr1, rwd1).Times(1)
	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, rwd1).Return(nil).Times(1)
	err = k.CalculateCostakerRewardsAndSendToGauge(ctx, costakr1, endedPeriod)
	require.NoError(t, err)

	ictvK.EXPECT().AccumulateRewardGaugeForCostaker(ctx, costakr2, rwd2).Times(1)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, rwd2).Return(nil).Times(1)
	err = k.CalculateCostakerRewardsAndSendToGauge(ctx, costakr2, endedPeriod)
	require.NoError(t, err)

	// Test the incremental reward scenario by creating period 2 historical rewards
	// that accumulate on top of period 1
	additionalRewards := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(50000)))
	additionalRewardsPerScore := additionalRewards.MulInt(ictvtypes.DecimalRewards).QuoInt(totalScore)

	endedPeriod++
	cumulativeRewardsPerScore := historical1.CumulativeRewardsPerScore.Add(additionalRewardsPerScore...)
	histPeriod2 := types.NewHistoricalRewards(cumulativeRewardsPerScore)
	err = k.setHistoricalRewards(ctx, endedPeriod, histPeriod2)
	require.NoError(t, err)

	// Calculate rewards for period 2 (should be total from both periods)
	rwd1Period2, err := k.CalculateCostakerRewards(ctx, costakr1, endedPeriod)
	require.NoError(t, err)

	rwd2Period2, err := k.CalculateCostakerRewards(ctx, costakr2, endedPeriod)
	require.NoError(t, err)

	// Expected rewards should be from both periods combined
	totalRewards := rewards.Add(additionalRewards...)
	expRwd1 := totalRewards.MulInt(costakr1Score).QuoInt(totalScore)
	expRwd2 := totalRewards.MulInt(costakr2Score).QuoInt(totalScore)

	require.True(t, expRwd1.IsAllLT(expRwd2))
	require.Equal(t, expRwd1.String(), rwd1Period2.String())
	require.Equal(t, expRwd2.String(), rwd2Period2.String())
}

func TestCostakerModifiedActiveAmounts(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

	dp := types.DefaultParams()
	dp.ScoreRatioBtcByBaby = sdkmath.NewInt(50)
	err := k.SetParams(ctx, dp)
	require.NoError(t, err)

	// costaker still doesn't exist
	costaker := datagen.GenRandomAddress()
	activeSats := sdkmath.NewInt(1000)
	activeBaby := sdkmath.NewInt(150)

	err = k.costakerModifiedActiveAmounts(ctx, costaker, activeSats, activeBaby)
	require.NoError(t, err)

	// min(1000, 150/50) = 3
	actCostaker, err := k.GetCostakerRewards(ctx, costaker)
	require.NoError(t, err)
	require.Equal(t, actCostaker.StartPeriodCumulativeReward, uint64(1)) // periods always starts at 1
	require.Equal(t, actCostaker.ActiveBaby, activeBaby)
	require.Equal(t, actCostaker.ActiveSatoshis, activeSats)
	require.Equal(t, actCostaker.TotalScore, sdkmath.NewInt(3))

	currRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currRwd.Period, uint64(2))
	require.Equal(t, currRwd.TotalScore, sdkmath.NewInt(3))
	require.Equal(t, currRwd.Rewards.String(), sdk.NewCoins().String())

	// simulate new active sats, but since it is less than the the previous the total score doesn't change
	// also the period doesn't need to change
	newActiveSats := sdkmath.NewInt(500)
	err = k.costakerModifiedActiveAmounts(ctx, costaker, newActiveSats, activeBaby)
	require.NoError(t, err)
	newActCostaker, err := k.GetCostakerRewards(ctx, costaker)
	require.NoError(t, err)
	require.Equal(t, newActCostaker.StartPeriodCumulativeReward, uint64(1))
	require.Equal(t, newActCostaker.ActiveBaby, activeBaby)
	require.Equal(t, newActCostaker.ActiveSatoshis, newActiveSats)
	require.Equal(t, newActCostaker.TotalScore, actCostaker.TotalScore)

	newCurrRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, newCurrRwd.Period, uint64(2))
	require.Equal(t, newCurrRwd.TotalScore, currRwd.TotalScore)
	require.Equal(t, newCurrRwd.Rewards.String(), currRwd.Rewards.String())

	// simulate a change in the baby and sats amount
	newActiveBaby := sdkmath.NewInt(45000)
	newActiveSats = sdkmath.NewInt(500)
	err = k.costakerModifiedActiveAmounts(ctx, costaker, newActiveSats, newActiveBaby)
	require.NoError(t, err)
	newActCostaker1, err := k.GetCostakerRewards(ctx, costaker)
	require.NoError(t, err)
	require.Equal(t, newActCostaker1.StartPeriodCumulativeReward, newCurrRwd.Period)
	require.Equal(t, newActCostaker1.ActiveBaby, newActiveBaby)
	require.Equal(t, newActCostaker1.ActiveSatoshis, newActiveSats)
	// min(500, 45000/50) = 500
	expTotalScore := sdkmath.NewInt(500)
	require.Equal(t, newActCostaker1.TotalScore, expTotalScore)

	// check again the current rewards
	newCurrRwd, err = k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, newCurrRwd.Period, uint64(3))
	require.Equal(t, newCurrRwd.TotalScore, expTotalScore)
	require.Equal(t, newCurrRwd.Rewards.String(), currRwd.Rewards.String())

	// Adds some rewards
	rwdCoins := datagen.GenRandomCoins(r)
	err = k.AddRewardsForCostakers(ctx, rwdCoins)
	require.NoError(t, err)

	// second costaker comes in with score of 250
	costakr2 := datagen.GenRandomAddress()
	activeSatsCo2 := sdkmath.NewInt(500)
	activeBabyCo2 := sdkmath.NewInt(12500)

	// min(500, 12500/50) = 250
	err = k.costakerModifiedActiveAmounts(ctx, costakr2, activeSatsCo2, activeBabyCo2)
	require.NoError(t, err)
	actCostaker2, err := k.GetCostakerRewards(ctx, costakr2)
	require.NoError(t, err)
	require.Equal(t, actCostaker2.StartPeriodCumulativeReward, uint64(3))
	require.Equal(t, actCostaker2.ActiveBaby, activeBabyCo2)
	require.Equal(t, actCostaker2.ActiveSatoshis, activeSatsCo2)
	require.Equal(t, actCostaker2.TotalScore, sdkmath.NewInt(250))

	// new period was created with empty rewards
	curRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, curRwd.Period, uint64(4))
	require.Equal(t, curRwd.TotalScore, actCostaker2.TotalScore.Add(newActCostaker1.TotalScore))
	require.Equal(t, curRwd.Rewards.String(), sdk.NewCoins().String())

	// the historical of prev period has the rewards
	hist, err := k.GetHistoricalRewards(ctx, curRwd.Period-1)
	require.NoError(t, err)

	histRwds := hist.CumulativeRewardsPerScore.MulInt(newActCostaker1.TotalScore).QuoInt(ictvtypes.DecimalRewards)
	coins.RequireCoinsDiffInPointOnePercentMargin(t, histRwds, rwdCoins)
}

func TestGetCostakerRewardsOrInitialize(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	costaker := datagen.GenRandomAddress()

	emptyInitialized, err := k.GetCostakerRewardsOrInitialize(ctx, costaker)
	require.NoError(t, err)
	require.Equal(t, emptyInitialized.StartPeriodCumulativeReward, uint64(0))
	require.Equal(t, emptyInitialized.ActiveBaby, sdkmath.ZeroInt())
	require.Equal(t, emptyInitialized.ActiveSatoshis, sdkmath.ZeroInt())
	require.Equal(t, emptyInitialized.TotalScore, sdkmath.ZeroInt())

	exp := datagen.GenRandomCostakerRewardsTracker(r)
	err = k.setCostakerRewardsTracker(ctx, costaker, exp)
	require.NoError(t, err)

	act, err := k.GetCostakerRewardsOrInitialize(ctx, costaker)
	require.NoError(t, err)
	require.Equal(t, act.StartPeriodCumulativeReward, exp.StartPeriodCumulativeReward)
	require.Equal(t, act.ActiveBaby.String(), exp.ActiveBaby.String())
	require.Equal(t, act.ActiveSatoshis.String(), exp.ActiveSatoshis.String())
	require.Equal(t, act.TotalScore.String(), exp.TotalScore.String())
}

func NewKeeperWithMockIncentiveKeeper(t *testing.T, mockIctvK types.IncentiveKeeper) (*Keeper, sdk.Context) {
	encConf := appparams.DefaultEncodingConfig()
	ctx, kvStore := store.NewStoreWithCtx(t, types.ModuleName)

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockBankK := types.NewMockBankKeeper(ctrl)
	mockAccK := types.NewMockAccountKeeper(ctrl)
	stkK := types.NewMockStakingKeeper(ctrl)
	dstrK := types.NewMockDistributionKeeper(ctrl)

	k := NewKeeper(
		encConf.Codec,
		kvStore,
		mockBankK,
		mockAccK,
		mockIctvK,
		stkK,
		dstrK,
		appparams.AccGov.String(),
		appparams.AccFeeCollector.String(),
	)

	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)
	return &k, ctx
}
