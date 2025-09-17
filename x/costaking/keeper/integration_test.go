package keeper

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestBankCoStakingModuleCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	costaker := datagen.GenRandomAddress()
	rewards := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100000)))

	// Set up proper historical rewards for the test to work
	startPeriod := uint64(0)
	err := k.setHistoricalRewards(ctx, startPeriod, types.NewHistoricalRewards(sdk.NewCoins()))
	require.NoError(t, err)

	endPeriod := startPeriod + 1
	rwdsPerScoreWithDecimals := rewards.MulInt(ictvtypes.DecimalRewards)
	err = k.setHistoricalRewards(ctx, endPeriod, types.NewHistoricalRewards(rwdsPerScoreWithDecimals))
	require.NoError(t, err)

	tracker := types.NewCostakerRewardsTrackerBasic(startPeriod, sdkmath.NewInt(1000))
	err = k.setCostakerRewardsTracker(ctx, costaker, tracker)
	require.NoError(t, err)

	// Calculate expected rewards: the historical rewards are stored with decimals multiplied
	// and the calculation will return rewards after dividing by DecimalRewards
	// expectedRewards = rwdsPerScoreWithDecimals * tracker.TotalScore / DecimalRewards
	expectedRewards := rwdsPerScoreWithDecimals.MulInt(tracker.TotalScore).QuoInt(ictvtypes.DecimalRewards)

	// Test that CalculateCostakerRewardsAndSendToGauge calls both:
	// 1. IncentiveKeeper.AccumulateRewardGaugeForCostaker
	// 2. BankKeeper.SendCoinsFromModuleToModule
	mockIctvK.EXPECT().AccumulateRewardGaugeForCostaker(gomock.Any(), costaker, expectedRewards).Times(1)

	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(
		gomock.Any(),
		gomock.Eq(types.ModuleName),
		gomock.Eq(ictvtypes.ModuleName),
		expectedRewards,
	).Return(nil).Times(1)

	err = k.CalculateCostakerRewardsAndSendToGauge(ctx, costaker, 1)
	require.NoError(t, err)
}

func TestBankModuleIntegrationWithZeroRewards(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	costakr := datagen.GenRandomAddress()

	emptyHist := types.NewHistoricalRewards(sdk.NewCoins())
	startPeriod := uint64(0)
	err := k.setHistoricalRewards(ctx, startPeriod, emptyHist)
	require.NoError(t, err)

	endPeriod := startPeriod + 1
	err = k.setHistoricalRewards(ctx, endPeriod, emptyHist)
	require.NoError(t, err)

	// Set up a tracker with zero score - should result in zero rewards
	tracker := types.NewCostakerRewardsTrackerBasic(startPeriod, sdkmath.ZeroInt())
	err = k.setCostakerRewardsTracker(ctx, costakr, tracker)
	require.NoError(t, err)

	// Without rewards no bank or incentives are called
	err = k.CalculateCostakerRewardsAndSendToGauge(ctx, costakr, endPeriod)
	require.NoError(t, err)
}

func TestBankModuleIntegrationFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	costakr := datagen.GenRandomAddress()
	rewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(50000)))

	// Create historical rewards to make the test work
	err := k.setHistoricalRewards(ctx, 0, types.NewHistoricalRewards(sdk.NewCoins()))
	require.NoError(t, err)
	err = k.setHistoricalRewards(ctx, 1, types.NewHistoricalRewards(rewards.MulInt(ictvtypes.DecimalRewards)))
	require.NoError(t, err)

	tracker := types.NewCostakerRewardsTrackerBasic(0, sdkmath.NewInt(1000))
	err = k.setCostakerRewardsTracker(ctx, costakr, tracker)
	require.NoError(t, err)

	// Calculate expected rewards: 50000 * 1000 = 50000000
	expectedRewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(50000000)))

	// Simulate bank transfer failure
	mockIctvK.EXPECT().AccumulateRewardGaugeForCostaker(ctx, costakr, expectedRewards).Times(1)

	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(
		ctx,
		types.ModuleName,
		ictvtypes.ModuleName,
		expectedRewards,
	).Return(errors.New("insufficient funds")).Times(1)

	// Should return the bank transfer error
	err = k.CalculateCostakerRewardsAndSendToGauge(ctx, costakr, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient funds")
}

func FuzzBankModuleIntegration(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockIctvK := types.NewMockIncentiveKeeper(ctrl)
		k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

		costakr := datagen.GenRandomAddress()
		costakrScore := datagen.RandomMathInt(r, 10000).AddRaw(1) // Ensure non-zero
		rewards := datagen.GenRandomCoins(r)

		if rewards.IsZero() {
			return // Skip test for zero rewards
		}

		// Create historical rewards
		rewardsWithDecimals := rewards.MulInt(ictvtypes.DecimalRewards)
		err := k.setHistoricalRewards(ctx, 0, types.NewHistoricalRewards(sdk.NewCoins()))
		require.NoError(t, err)
		err = k.setHistoricalRewards(ctx, 1, types.NewHistoricalRewards(rewardsWithDecimals))
		require.NoError(t, err)

		tracker := types.NewCostakerRewardsTrackerBasic(0, costakrScore)
		err = k.setCostakerRewardsTracker(ctx, costakr, tracker)
		require.NoError(t, err)

		// Calculate expected rewards
		expectedRewards := rewardsWithDecimals.MulInt(costakrScore).QuoInt(ictvtypes.DecimalRewards)

		if !expectedRewards.IsZero() {
			// Both calls should happen for non-zero rewards
			mockIctvK.EXPECT().AccumulateRewardGaugeForCostaker(ctx, costakr, expectedRewards).Times(1)

			mockBankK := k.bankK.(*types.MockBankKeeper)
			mockBankK.EXPECT().SendCoinsFromModuleToModule(
				ctx,
				types.ModuleName,
				ictvtypes.ModuleName,
				expectedRewards,
			).Return(nil).Times(1)
		}

		err = k.CalculateCostakerRewardsAndSendToGauge(ctx, costakr, 1)
		require.NoError(t, err)
	})
}

func TestCostakerWithdrawRewards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ictvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)

	costaker := datagen.GenRandomAddress()

	_, err := k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	// Set up initial tracker and rewards
	initialScore := sdkmath.NewInt(1000)
	tracker := types.NewCostakerRewardsTrackerBasic(0, initialScore)
	err = k.setCostakerRewardsTracker(ctx, costaker, tracker)
	require.NoError(t, err)

	err = k.UpdateCurrentRewardsTotalScore(ctx, initialScore)
	require.NoError(t, err)

	// Add some rewards to the system
	rewards := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100000)))
	err = k.AddRewardsForCostakers(ctx, rewards)
	require.NoError(t, err)

	// Calculate expected rewards: rewards should equal the original amount since score == totalScore
	// When IncrementRewardsPeriod is called, it divides rewards by totalScore to get rewardsPerScore
	// Then reward calculation multiplies rewardsPerScore by costakr score
	expectedRewards := rewards

	// Test costakerModified - this should:
	// 1. Call IncrementRewardsPeriod
	// 2. Calculate and send rewards to gauge (both incentive + bank)
	// 3. Reinitialize the tracker

	ictvK.EXPECT().AccumulateRewardGaugeForCostaker(ctx, costaker, expectedRewards).Times(1)

	bankK := k.bankK.(*types.MockBankKeeper)
	bankK.EXPECT().SendCoinsFromModuleToModule(
		gomock.Any(),
		gomock.Eq(types.ModuleName),
		gomock.Eq(ictvtypes.ModuleName),
		gomock.Eq(expectedRewards),
	).Return(nil).Times(1)

	// Call the function being tested
	err = k.costakerWithdrawRewards(ctx, costaker)
	require.NoError(t, err)

	updatedTracker, err := k.GetCostakerRewards(ctx, costaker)
	require.NoError(t, err)

	currentRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	expectedStartPeriod := currentRwd.Period - 1

	require.Equal(t, expectedStartPeriod, updatedTracker.StartPeriodCumulativeReward)
	require.Equal(t, initialScore.String(), updatedTracker.TotalScore.String())
}

func TestCostakerModifiedWithPreInitialization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ictvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)

	costaker := datagen.GenRandomAddress()

	_, err := k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	initialScore := sdkmath.NewInt(500)
	newScore := sdkmath.NewInt(750)

	tracker := types.NewCostakerRewardsTrackerBasic(0, initialScore)
	err = k.setCostakerRewardsTracker(ctx, costaker, tracker)
	require.NoError(t, err)

	err = k.UpdateCurrentRewardsTotalScore(ctx, initialScore)
	require.NoError(t, err)

	rewards := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(50000)))
	err = k.AddRewardsForCostakers(ctx, rewards)
	require.NoError(t, err)

	// Expected rewards based on initial score: since score == totalScore, rewards should equal the original amount
	expectedRewards := rewards

	// Mock the reward distribution
	ictvK.EXPECT().AccumulateRewardGaugeForCostaker(ctx, costaker, expectedRewards).Times(1)

	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(
		gomock.Any(),
		gomock.Eq(types.ModuleName),
		gomock.Eq(ictvtypes.ModuleName),
		gomock.Eq(expectedRewards),
	).Return(nil).Times(1)

	// Pre-initialization function that modifies the score
	preInitFunc := func(ctx context.Context, costakr sdk.AccAddress) error {
		currentTracker, err := k.GetCostakerRewards(ctx, costakr)
		if err != nil {
			return err
		}

		updatedTracker := types.NewCostakerRewardsTrackerBasic(
			currentTracker.StartPeriodCumulativeReward,
			newScore,
		)
		currentTracker.TotalScore = newScore
		return k.setCostakerRewardsTracker(ctx, costakr, updatedTracker)
	}

	// Call with pre-initialization
	err = k.costakerModifiedScoreWithPreInitalization(ctx, costaker, preInitFunc)
	require.NoError(t, err)

	updatedTracker, err := k.GetCostakerRewards(ctx, costaker)
	require.NoError(t, err)

	require.Equal(t, newScore.String(), updatedTracker.TotalScore.String())
}
