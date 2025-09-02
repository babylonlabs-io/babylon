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
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func FuzzAddRewardsForCoostakers(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockIctvK := types.NewMockIncentiveKeeper(ctrl)
		k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

		rewards := datagen.GenRandomCoins(r)

		err := k.AddRewardsForCoostakers(ctx, rewards)
		require.NoError(t, err)

		currentRwd, err := k.GetCurrentRewards(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(1), currentRwd.Period)
		require.Equal(t, rewards.MulInt(ictvtypes.DecimalRewards).String(), currentRwd.Rewards.String())
		require.Equal(t, sdkmath.ZeroInt().String(), currentRwd.TotalScore.String())

		additionalRewards := datagen.GenRandomCoins(r)
		err = k.AddRewardsForCoostakers(ctx, additionalRewards)
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

		err = k.AddRewardsForCoostakers(ctx, rewards)
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

func FuzzCalculateCoostakerRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

		coostaker := datagen.GenRandomAddress()
		coostakerScore := datagen.RandomMathInt(r, 10000).AddRaw(10)

		hist1 := datagen.GenRandomHistoricalRewards(r)
		hist1.CumulativeRewardsPerScore = hist1.CumulativeRewardsPerScore.MulInt(ictvtypes.DecimalRewards)
		startPeriod := datagen.RandomInt(r, 10)

		err := k.setHistoricalRewards(ctx, startPeriod, hist1)
		require.NoError(t, err)

		hist2 := types.NewHistoricalRewards(hist1.CumulativeRewardsPerScore.MulInt(sdkmath.NewInt(2)))
		endPeriod := startPeriod + 1 + datagen.RandomInt(r, 10)
		err = k.setHistoricalRewards(ctx, endPeriod, hist2)
		require.NoError(t, err)

		initialTracker := types.NewCoostakerRewardsTrackerBasic(startPeriod, coostakerScore)
		err = k.setCoostakerRewardsTracker(ctx, coostaker, initialTracker)
		require.NoError(t, err)

		expRwds := hist2.CumulativeRewardsPerScore.Sub(hist1.CumulativeRewardsPerScore...).MulInt(coostakerScore).QuoInt(ictvtypes.DecimalRewards)
		rewards, err := k.CalculateCoostakerRewards(ctx, coostaker, endPeriod)
		require.NoError(t, err)
		require.Equal(t, expRwds.String(), rewards.String())
	})
}

func FuzzCalculateCoostakerRewardsAndSendToGauge(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ictvK := types.NewMockIncentiveKeeper(ctrl)
		k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)

		coostaker := datagen.GenRandomAddress()
		coostakerScore := datagen.RandomMathInt(r, 10000).AddRaw(10)

		hist1 := datagen.GenRandomHistoricalRewards(r)
		hist1.CumulativeRewardsPerScore = hist1.CumulativeRewardsPerScore.MulInt(ictvtypes.DecimalRewards)
		startPeriod := datagen.RandomInt(r, 10)

		err := k.setHistoricalRewards(ctx, startPeriod, hist1)
		require.NoError(t, err)

		hist2 := types.NewHistoricalRewards(hist1.CumulativeRewardsPerScore.MulInt(sdkmath.NewInt(2)))
		endPeriod := startPeriod + 1 + datagen.RandomInt(r, 10)
		err = k.setHistoricalRewards(ctx, endPeriod, hist2)
		require.NoError(t, err)

		initialTracker := types.NewCoostakerRewardsTrackerBasic(startPeriod, coostakerScore)
		err = k.setCoostakerRewardsTracker(ctx, coostaker, initialTracker)
		require.NoError(t, err)

		expRwds := hist2.CumulativeRewardsPerScore.Sub(hist1.CumulativeRewardsPerScore...).MulInt(coostakerScore).QuoInt(ictvtypes.DecimalRewards)

		ictvK.EXPECT().AccumulateRewardGaugeForCoostaker(
			gomock.Any(),
			gomock.Eq(coostaker),
			gomock.Eq(expRwds),
		).Times(1)

		bankK := k.bankK.(*types.MockBankKeeper)
		bankK.EXPECT().SendCoinsFromModuleToModule(
			gomock.Any(),
			gomock.Eq(types.ModuleName),
			gomock.Eq(ictvtypes.ModuleName),
			gomock.Eq(expRwds),
		).Return(nil).Times(1)

		err = k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker, endPeriod)
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

func TestCalculateCoostakerRewardsBetweenNegativeRewards(t *testing.T) {
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

	tracker := types.NewCoostakerRewardsTrackerBasic(startPeriod, sdkmath.NewInt(100))

	delta, _ := endingRewards.SafeSub(startingRewards...)
	_, err = k.calculateCoStakerRewardsBetween(ctx, tracker, endPeriod)
	require.EqualError(t, err, types.ErrNegativeRewards.Wrapf("cumulative rewards is negative %s", delta.String()).Error())
}

func TestCalculateCoostakerRewardsBetweenInvalidPeriod(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

	startPeriod := uint64(10)
	tracker := types.NewCoostakerRewardsTrackerBasic(startPeriod, sdkmath.NewInt(100))

	endPeriod := uint64(5)
	_, err := k.calculateCoStakerRewardsBetween(ctx, tracker, endPeriod)
	require.EqualError(t, err, types.ErrInvalidPeriod.Wrapf("startingPeriod %d cannot be greater than endingPeriod %d", startPeriod, endPeriod).Error())
}

func TestInitializeCoostakerRwdTracker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	coostaker := datagen.GenRandomAddress()
	totalScore := sdkmath.NewInt(1000)

	initialTracker := types.NewCoostakerRewardsTrackerBasic(0, totalScore)
	err := k.setCoostakerRewardsTracker(ctx, coostaker, initialTracker)
	require.NoError(t, err)

	r := rand.New(rand.NewSource(42))
	err = k.AddRewardsForCoostakers(ctx, datagen.GenRandomCoins(r))
	require.NoError(t, err)

	endedPeriod, err := k.IncrementRewardsPeriod(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), endedPeriod)

	err = k.initializeCoStakerRwdTracker(ctx, coostaker)
	require.NoError(t, err)

	updatedTracker, err := k.GetCoostakerRewards(ctx, coostaker)
	require.NoError(t, err)

	currentRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	expectedStartPeriod := currentRwd.Period - 1

	require.Equal(t, expectedStartPeriod, updatedTracker.StartPeriodCumulativeReward)
	require.Equal(t, totalScore.String(), updatedTracker.TotalScore.String())
}

func TestCoostakerRewardsFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ictvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)

	coostaker1 := datagen.GenRandomAddress()
	coostaker2 := datagen.GenRandomAddress()

	coostaker1Score := sdkmath.NewInt(300)
	coostaker2Score := sdkmath.NewInt(700)
	totalScore := coostaker1Score.Add(coostaker2Score)

	// Test the full flow by manually setting up historical rewards like the working tests do
	// Initialize the rewards tracking system first
	_, err := k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	tracker1 := types.NewCoostakerRewardsTrackerBasic(0, coostaker1Score)
	tracker2 := types.NewCoostakerRewardsTrackerBasic(0, coostaker2Score)

	err = k.setCoostakerRewardsTracker(ctx, coostaker1, tracker1)
	require.NoError(t, err)
	err = k.setCoostakerRewardsTracker(ctx, coostaker2, tracker2)
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
	rwd1, err := k.CalculateCoostakerRewards(ctx, coostaker1, endedPeriod)
	require.NoError(t, err)

	rwd2, err := k.CalculateCoostakerRewards(ctx, coostaker2, endedPeriod)
	require.NoError(t, err)

	// Expected rewards should be proportional to their scores
	expRwdCoostaker1 := rewards.MulInt(coostaker1Score).QuoInt(totalScore)
	expRwdCoostaker2 := rewards.MulInt(coostaker2Score).QuoInt(totalScore)

	require.Equal(t, expRwdCoostaker1.String(), rwd1.String())
	require.Equal(t, expRwdCoostaker2.String(), rwd2.String())

	// Mock expectations for both incentive gauge accumulation and bank transfer
	ictvK.EXPECT().AccumulateRewardGaugeForCoostaker(ctx, coostaker1, rwd1).Times(1)
	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, rwd1).Return(nil).Times(1)
	err = k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker1, endedPeriod)
	require.NoError(t, err)

	ictvK.EXPECT().AccumulateRewardGaugeForCoostaker(ctx, coostaker2, rwd2).Times(1)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, rwd2).Return(nil).Times(1)
	err = k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker2, endedPeriod)
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
	rwd1Period2, err := k.CalculateCoostakerRewards(ctx, coostaker1, endedPeriod)
	require.NoError(t, err)

	rwd2Period2, err := k.CalculateCoostakerRewards(ctx, coostaker2, endedPeriod)
	require.NoError(t, err)

	// Expected rewards should be from both periods combined
	totalRewards := rewards.Add(additionalRewards...)
	expRwd1 := totalRewards.MulInt(coostaker1Score).QuoInt(totalScore)
	expRwd2 := totalRewards.MulInt(coostaker2Score).QuoInt(totalScore)

	require.True(t, expRwd1.IsAllLT(expRwd2))
	require.Equal(t, expRwd1.String(), rwd1Period2.String())
	require.Equal(t, expRwd2.String(), rwd2Period2.String())
}

func TestCoostakerModifiedActiveAmounts(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

	dp := types.DefaultParams()
	dp.ScoreRatioBtcByBaby = sdkmath.NewInt(50)
	err := k.SetParams(ctx, dp)
	require.NoError(t, err)

	// coostaker still doesn't exist
	coostaker := datagen.GenRandomAddress()
	activeSats := sdkmath.NewInt(1000)
	activeBaby := sdkmath.NewInt(150)

	err = k.coostakerModifiedActiveAmounts(ctx, coostaker, activeSats, activeBaby)
	require.NoError(t, err)

	// min(1000, 150/50) = 3
	actCoostaker, err := k.GetCoostakerRewards(ctx, coostaker)
	require.NoError(t, err)
	require.Equal(t, actCoostaker.StartPeriodCumulativeReward, uint64(1)) // periods always starts at 1
	require.Equal(t, actCoostaker.ActiveBaby, activeBaby)
	require.Equal(t, actCoostaker.ActiveSatoshis, activeSats)
	require.Equal(t, actCoostaker.TotalScore, sdkmath.NewInt(3))

	currRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currRwd.Period, uint64(2))
	require.Equal(t, currRwd.TotalScore, sdkmath.NewInt(3))
	require.Equal(t, currRwd.Rewards.String(), sdk.NewCoins().String())

	// simulate new active sats, but since it is less than the the previous the total score doesn't change
	newActiveSats := sdkmath.NewInt(500)
	err = k.coostakerModifiedActiveAmounts(ctx, coostaker, newActiveSats, activeBaby)
	require.NoError(t, err)
	newActCoostaker, err := k.GetCoostakerRewards(ctx, coostaker)
	require.NoError(t, err)
	require.Equal(t, newActCoostaker.StartPeriodCumulativeReward, currRwd.Period)
	require.Equal(t, newActCoostaker.ActiveBaby, activeBaby)
	require.Equal(t, newActCoostaker.ActiveSatoshis, newActiveSats)
	require.Equal(t, newActCoostaker.TotalScore, actCoostaker.TotalScore)

	newCurrRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, newCurrRwd.Period, uint64(3))
	require.Equal(t, newCurrRwd.TotalScore, currRwd.TotalScore)
	require.Equal(t, newCurrRwd.Rewards.String(), currRwd.Rewards.String())

	// simulate a change in the baby and sats amount
	newActiveBaby := sdkmath.NewInt(45000)
	newActiveSats = sdkmath.NewInt(500)
	err = k.coostakerModifiedActiveAmounts(ctx, coostaker, newActiveSats, newActiveBaby)
	require.NoError(t, err)
	newActCoostaker1, err := k.GetCoostakerRewards(ctx, coostaker)
	require.NoError(t, err)
	require.Equal(t, newActCoostaker1.StartPeriodCumulativeReward, newCurrRwd.Period)
	require.Equal(t, newActCoostaker1.ActiveBaby, newActiveBaby)
	require.Equal(t, newActCoostaker1.ActiveSatoshis, newActiveSats)
	// min(500, 45000/50) = 500
	expTotalScore := sdkmath.NewInt(500)
	require.Equal(t, newActCoostaker1.TotalScore, expTotalScore)

	// check again the current rewards
	newCurrRwd, err = k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, newCurrRwd.Period, uint64(4))
	require.Equal(t, newCurrRwd.TotalScore, expTotalScore)
	require.Equal(t, newCurrRwd.Rewards.String(), currRwd.Rewards.String())

	// Adds some rewards
	rwdCoins := datagen.GenRandomCoins(r)
	err = k.AddRewardsForCoostakers(ctx, rwdCoins)
	require.NoError(t, err)

	// second coostaker comes in with score of 250
	coostaker2 := datagen.GenRandomAddress()
	activeSatsCo2 := sdkmath.NewInt(500)
	activeBabyCo2 := sdkmath.NewInt(12500)

	// min(500, 12500/50) = 250
	err = k.coostakerModifiedActiveAmounts(ctx, coostaker2, activeSatsCo2, activeBabyCo2)
	require.NoError(t, err)
	actCostaker2, err := k.GetCoostakerRewards(ctx, coostaker2)
	require.NoError(t, err)
	require.Equal(t, actCostaker2.StartPeriodCumulativeReward, uint64(4))
	require.Equal(t, actCostaker2.ActiveBaby, activeBabyCo2)
	require.Equal(t, actCostaker2.ActiveSatoshis, activeSatsCo2)
	require.Equal(t, actCostaker2.TotalScore, sdkmath.NewInt(250))

	// new period was created with empty rewards
	curRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, curRwd.Period, uint64(5))
	require.Equal(t, curRwd.TotalScore, actCostaker2.TotalScore.Add(newActCoostaker1.TotalScore))
	require.Equal(t, curRwd.Rewards.String(), sdk.NewCoins().String())

	// the historical of prev period has the rewards
	hist, err := k.GetHistoricalRewards(ctx, curRwd.Period-1)
	require.NoError(t, err)

	histRwds := hist.CumulativeRewardsPerScore.MulInt(newActCoostaker1.TotalScore).QuoInt(ictvtypes.DecimalRewards)
	coins.RequireCoinsDiffInPointOnePercentMargin(t, histRwds, rwdCoins)
}

func TestGetCoostakerRewardsOrInitialize(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	coostaker := datagen.GenRandomAddress()

	emptyInitialized, err := k.GetCoostakerRewardsOrInitialize(ctx, coostaker)
	require.NoError(t, err)
	require.Equal(t, emptyInitialized.StartPeriodCumulativeReward, uint64(0))
	require.Equal(t, emptyInitialized.ActiveBaby, sdkmath.ZeroInt())
	require.Equal(t, emptyInitialized.ActiveSatoshis, sdkmath.ZeroInt())
	require.Equal(t, emptyInitialized.TotalScore, sdkmath.ZeroInt())

	exp := datagen.GenRandomCoostakerRewardsTracker(r)
	err = k.setCoostakerRewardsTracker(ctx, coostaker, exp)
	require.NoError(t, err)

	act, err := k.GetCoostakerRewardsOrInitialize(ctx, coostaker)
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
	return &k, ctx
}
