package keeper

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestBankModuleIntegration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	coostaker := datagen.GenRandomAddress()
	rewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100000)))

	// Set up proper historical rewards for the test to work
	err := k.setHistoricalRewards(ctx, 0, types.NewHistoricalRewards(sdk.NewCoins()))
	require.NoError(t, err)
	err = k.setHistoricalRewards(ctx, 1, types.NewHistoricalRewards(rewards.MulInt(ictvtypes.DecimalRewards)))
	require.NoError(t, err)

	// Set up tracker
	tracker := types.NewCoostakerRewardsTracker(0, sdkmath.NewInt(1000))
	err = k.setCoostakerRewardsTracker(ctx, coostaker, tracker)
	require.NoError(t, err)

	// Calculate expected rewards: the historical rewards are stored with decimals multiplied
	// and the calculation will return rewards after dividing by DecimalRewards
	// rewards = rewardsWithDecimals * score / DecimalRewards
	// rewardsWithDecimals = rewards.MulInt(DecimalRewards) = 100000 * 10^20
	// expectedRewards = rewardsWithDecimals * 1000 / DecimalRewards = (100000 * 10^20 * 1000) / 10^20 = 100000 * 1000 = 100000000
	expectedRewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100000000)))

	// Test that CalculateCoostakerRewardsAndSendToGauge calls both:
	// 1. IncentiveKeeper.AccumulateRewardGaugeForCoostaker 
	// 2. BankKeeper.SendCoinsFromModuleToModule
	mockIctvK.EXPECT().AccumulateRewardGaugeForCoostaker(ctx, coostaker, expectedRewards).Times(1)
	
	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(
		ctx, 
		types.ModuleName,        // from: coostaking module
		ictvtypes.ModuleName,    // to: incentive module  
		expectedRewards,         // amount
	).Return(nil).Times(1)

	// This should trigger both the gauge accumulation and bank transfer
	err = k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker, 1)
	require.NoError(t, err) // Should succeed now with proper setup
}

func TestBankModuleIntegrationWithZeroRewards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	coostaker := datagen.GenRandomAddress()
	
	// Set up historical rewards for the test to work
	err := k.setHistoricalRewards(ctx, 0, types.NewHistoricalRewards(sdk.NewCoins()))
	require.NoError(t, err)
	err = k.setHistoricalRewards(ctx, 1, types.NewHistoricalRewards(sdk.NewCoins()))
	require.NoError(t, err)
	
	// Set up a tracker with zero score - should result in zero rewards
	tracker := types.NewCoostakerRewardsTracker(0, sdkmath.ZeroInt())
	err = k.setCoostakerRewardsTracker(ctx, coostaker, tracker)
	require.NoError(t, err)

	// With zero rewards, neither incentive accumulation nor bank transfer should be called
	// (no expectations set means gomock will fail if they're called)

	err = k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker, 1)
	require.NoError(t, err) // Should succeed with zero rewards
}

func TestBankModuleIntegrationFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	coostaker := datagen.GenRandomAddress()
	rewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(50000)))
	
	// Create historical rewards to make the test work
	err := k.setHistoricalRewards(ctx, 0, types.NewHistoricalRewards(sdk.NewCoins()))
	require.NoError(t, err)
	err = k.setHistoricalRewards(ctx, 1, types.NewHistoricalRewards(rewards.MulInt(ictvtypes.DecimalRewards)))
	require.NoError(t, err)

	tracker := types.NewCoostakerRewardsTracker(0, sdkmath.NewInt(1000))
	err = k.setCoostakerRewardsTracker(ctx, coostaker, tracker)
	require.NoError(t, err)

	// Calculate expected rewards: 50000 * 1000 = 50000000
	expectedRewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(50000000)))

	// Simulate bank transfer failure
	mockIctvK.EXPECT().AccumulateRewardGaugeForCoostaker(ctx, coostaker, expectedRewards).Times(1)
	
	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(
		ctx, 
		types.ModuleName,
		ictvtypes.ModuleName, 
		expectedRewards,
	).Return(errors.New("insufficient funds")).Times(1)

	// Should return the bank transfer error
	err = k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker, 1)
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

		coostaker := datagen.GenRandomAddress()
		coostakerScore := datagen.RandomMathInt(r, 10000).AddRaw(1) // Ensure non-zero
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

		tracker := types.NewCoostakerRewardsTracker(0, coostakerScore)
		err = k.setCoostakerRewardsTracker(ctx, coostaker, tracker)
		require.NoError(t, err)

		// Calculate expected rewards
		expectedRewards := rewardsWithDecimals.MulInt(coostakerScore).QuoInt(ictvtypes.DecimalRewards)
		
		if !expectedRewards.IsZero() {
			// Both calls should happen for non-zero rewards
			mockIctvK.EXPECT().AccumulateRewardGaugeForCoostaker(ctx, coostaker, expectedRewards).Times(1)
			
			mockBankK := k.bankK.(*types.MockBankKeeper)
			mockBankK.EXPECT().SendCoinsFromModuleToModule(
				ctx,
				types.ModuleName,
				ictvtypes.ModuleName,
				expectedRewards,
			).Return(nil).Times(1)
		}

		err = k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker, 1)
		require.NoError(t, err)
	})
}

func TestCoostakerModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	coostaker := datagen.GenRandomAddress()
	
	// Initialize system first
	_, err := k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	// Set up initial tracker and rewards
	initialScore := sdkmath.NewInt(1000)
	tracker := types.NewCoostakerRewardsTracker(0, initialScore)
	err = k.setCoostakerRewardsTracker(ctx, coostaker, tracker)
	require.NoError(t, err)

	// Add some rewards to the system
	rewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100000)))
	err = k.AddRewardsForCoostakers(ctx, rewards)
	require.NoError(t, err)
	
	err = k.UpdateCurrentRewardsTotalScore(ctx, initialScore)
	require.NoError(t, err)

	// Calculate expected rewards: rewards should equal the original amount since score == totalScore
	// When IncrementRewardsPeriod is called, it divides rewards by totalScore to get rewardsPerScore
	// Then reward calculation multiplies rewardsPerScore by coostaker score
	// Since score == totalScore: reward = (rewards / totalScore) * score = rewards
	expectedRewards := rewards

	// Test coostakerModified - this should:
	// 1. Call IncrementRewardsPeriod
	// 2. Calculate and send rewards to gauge (both incentive + bank)
	// 3. Reinitialize the tracker
	
	// Expect the reward distribution calls
	mockIctvK.EXPECT().AccumulateRewardGaugeForCoostaker(ctx, coostaker, expectedRewards).Times(1)
	
	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(
		ctx,
		types.ModuleName,
		ictvtypes.ModuleName,
		expectedRewards,
	).Return(nil).Times(1)

	// Call the function being tested
	err = k.coostakerModified(ctx, coostaker)
	require.NoError(t, err)

	// Verify the tracker was reinitialized
	updatedTracker, err := k.GetCoostakerRewards(ctx, coostaker)
	require.NoError(t, err)
	
	// After modification, the tracker should start from the previous period
	currentRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	expectedStartPeriod := currentRwd.Period - 1
	
	require.Equal(t, expectedStartPeriod, updatedTracker.StartPeriodCumulativeReward)
	require.Equal(t, initialScore.String(), updatedTracker.TotalScore.String())
}

func TestCoostakerModifiedWithPreInitialization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	coostaker := datagen.GenRandomAddress()
	
	// Initialize system
	_, err := k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	// Set up tracker and rewards
	initialScore := sdkmath.NewInt(500)
	newScore := sdkmath.NewInt(750) // Will be set in pre-init function
	
	tracker := types.NewCoostakerRewardsTracker(0, initialScore)
	err = k.setCoostakerRewardsTracker(ctx, coostaker, tracker)
	require.NoError(t, err)

	rewards := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(50000)))
	err = k.AddRewardsForCoostakers(ctx, rewards)
	require.NoError(t, err)
	
	err = k.UpdateCurrentRewardsTotalScore(ctx, initialScore)
	require.NoError(t, err)

	// Expected rewards based on initial score: since score == totalScore, rewards should equal the original amount
	expectedRewards := rewards

	// Mock the reward distribution
	mockIctvK.EXPECT().AccumulateRewardGaugeForCoostaker(ctx, coostaker, expectedRewards).Times(1)
	
	mockBankK := k.bankK.(*types.MockBankKeeper)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(
		ctx,
		types.ModuleName,
		ictvtypes.ModuleName,
		expectedRewards,
	).Return(nil).Times(1)

	// Pre-initialization function that modifies the score
	preInitFunc := func(ctx context.Context, coostaker sdk.AccAddress) error {
		// This simulates updating the coostaker's score before tracker reinitialization
		currentTracker, err := k.GetCoostakerRewards(ctx, coostaker)
		if err != nil {
			return err
		}
		
		// Update the tracker with new score
		updatedTracker := types.NewCoostakerRewardsTracker(
			currentTracker.StartPeriodCumulativeReward, 
			newScore,
		)
		return k.setCoostakerRewardsTracker(ctx, coostaker, updatedTracker)
	}

	// Call with pre-initialization
	err = k.coostakerModifiedWithPreInitalization(ctx, coostaker, preInitFunc)
	require.NoError(t, err)

	// Verify the tracker was reinitialized with the new score
	updatedTracker, err := k.GetCoostakerRewards(ctx, coostaker)
	require.NoError(t, err)
	
	require.Equal(t, newScore.String(), updatedTracker.TotalScore.String())
}