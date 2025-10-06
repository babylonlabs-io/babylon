package keeper

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/coins"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

func TestMsgUpdateParamsUpdateAllCostakersScore(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ictvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)

	bankK := k.bankK.(*types.MockBankKeeper)
	// Setup three costakers
	// 1 => 1000 score
	// 2 => 2000 score
	// 3 => 0 score
	costk1 := datagen.GenRandomAddress()
	costk2 := datagen.GenRandomAddress()
	costk3 := datagen.GenRandomAddress()
	initialRatio := sdkmath.NewInt(50)

	dp := types.DefaultParams()
	dp.ScoreRatioBtcByBaby = initialRatio
	err := k.SetParams(ctx, dp)
	require.NoError(t, err)

	// Costaker 1: 5000 sats, 50000 baby -> score = min(5000, 50000/50) = 1000
	err = k.costakerModifiedActiveAmounts(ctx, costk1, sdkmath.NewInt(5000), sdkmath.NewInt(50000))
	require.NoError(t, err)

	// Costaker 2: 10000 sats, 100000 baby -> score = min(10000, 100000/50) = 2000
	err = k.costakerModifiedActiveAmounts(ctx, costk2, sdkmath.NewInt(10000), sdkmath.NewInt(100000))
	require.NoError(t, err)

	// Costaker 3: 5000 sats, 0 baby -> score = min(5000, 0/50) = 0
	err = k.costakerModifiedActiveAmounts(ctx, costk3, sdkmath.NewInt(5000), sdkmath.ZeroInt())
	require.NoError(t, err)

	// Verify total score: 3000
	currentRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(3000), currentRwd.TotalScore)

	// Deposit 30,000 tokens as rewards
	totalDeposited := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(30000)))
	err = k.AddRewardsForCostakers(ctx, totalDeposited)
	require.NoError(t, err)

	// Finalize the period so rewards becomes claimable
	periodBeforeUpdate, err := k.IncrementRewardsPeriod(ctx)
	require.NoError(t, err)

	// BEFORE UPDATE: Calculate total claimable rewards
	cost1RewardsBefore, err := k.CalculateCostakerRewards(ctx, costk1, periodBeforeUpdate)
	require.NoError(t, err)
	cost2RewardsBefore, err := k.CalculateCostakerRewards(ctx, costk2, periodBeforeUpdate)
	require.NoError(t, err)
	cost3RewardsBefore, err := k.CalculateCostakerRewards(ctx, costk3, periodBeforeUpdate)
	require.NoError(t, err)
	require.True(t, cost3RewardsBefore.IsZero())

	totalClaimableBefore := cost1RewardsBefore.Add(cost2RewardsBefore...)
	require.Equal(t, totalDeposited.String(), totalClaimableBefore.String())

	// GOVERNANCE UPDATES PARAMETER
	// This changes the ratio from 50 to 25, doubling scores for these costakers
	newRatio := sdkmath.NewInt(25)
	t.Logf("\n=== GOVERNANCE PARAMETER UPDATE ===")
	t.Logf("ScoreRatioBtcByBaby: 50 -> 25")

	ictvK.EXPECT().AccumulateRewardGaugeForCostaker(gomock.Any(), costk1, cost1RewardsBefore).Times(1)
	bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), types.ModuleName, ictvtypes.ModuleName, cost1RewardsBefore).Times(1)
	ictvK.EXPECT().AccumulateRewardGaugeForCostaker(gomock.Any(), costk2, cost2RewardsBefore).Times(1)
	bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), types.ModuleName, ictvtypes.ModuleName, cost2RewardsBefore).Times(1)

	err = k.UpdateAllCostakersScore(ctx, newRatio)
	require.NoError(t, err)
	currentStartCumulativeRewardPeriod := periodBeforeUpdate + 1

	// Verify scores doubled
	costk1TrackerAfter, err := k.GetCostakerRewards(ctx, costk1)
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(2000), costk1TrackerAfter.TotalScore, "score should double")
	require.Equal(t, currentStartCumulativeRewardPeriod, costk1TrackerAfter.StartPeriodCumulativeReward)

	costk2TrackerAfter, err := k.GetCostakerRewards(ctx, costk2)
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(4000), costk2TrackerAfter.TotalScore, "score should double")
	require.Equal(t, currentStartCumulativeRewardPeriod, costk2TrackerAfter.StartPeriodCumulativeReward)

	costk3TrackerAfter, err := k.GetCostakerRewards(ctx, costk3)
	require.NoError(t, err)
	require.True(t, costk3TrackerAfter.TotalScore.IsZero())
	require.Equal(t, periodBeforeUpdate, costk3TrackerAfter.StartPeriodCumulativeReward, "as costaker 3 didn't have any score, there was no need to change his start cumulative reward ratio")

	currentRwd, err = k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currentRwd.TotalScore.String(), costk1TrackerAfter.TotalScore.Add(costk2TrackerAfter.TotalScore).String())

	// AFTER UPDATE: calculate the reward tracker with previous period should error as the period was incremented
	costk1RewardsAfter, err := k.CalculateCostakerRewards(ctx, costk1, periodBeforeUpdate)
	require.EqualError(t, err, types.ErrInvalidPeriod.Wrapf("startingPeriod %d cannot be greater than endingPeriod %d", costk1TrackerAfter.StartPeriodCumulativeReward, periodBeforeUpdate).Error())
	costk2RewardsAfter, err := k.CalculateCostakerRewards(ctx, costk2, periodBeforeUpdate)
	require.EqualError(t, err, types.ErrInvalidPeriod.Wrapf("startingPeriod %d cannot be greater than endingPeriod %d", costk2TrackerAfter.StartPeriodCumulativeReward, periodBeforeUpdate).Error())
	costk3RewardsAfter, err := k.CalculateCostakerRewards(ctx, costk3, periodBeforeUpdate)
	require.NoError(t, err)
	require.True(t, costk3RewardsAfter.IsZero())

	totalClaimableAfter := costk1RewardsAfter.Add(costk2RewardsAfter...)
	require.True(t, totalClaimableAfter.IsZero(), "After updating params, all costaker reward trackers have their start period set to the current period, so no rewards can be claimed for previous periods; thus, total claimable rewards should be zero.")

	// after the update deposits it again, increment period and check the rewards of each one.
	totalDeposited = sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(50000)))
	err = k.AddRewardsForCostakers(ctx, totalDeposited)
	require.NoError(t, err)

	// Finalize the period so rewards becomes claimable
	finalizedPeriod, err := k.IncrementRewardsPeriod(ctx)
	require.NoError(t, err)

	// BEFORE UPDATE: Calculate total claimable rewards
	cost1Rewards, err := k.CalculateCostakerRewards(ctx, costk1, finalizedPeriod)
	require.NoError(t, err)
	cost2Rewards, err := k.CalculateCostakerRewards(ctx, costk2, finalizedPeriod)
	require.NoError(t, err)
	cost3Rewards, err := k.CalculateCostakerRewards(ctx, costk3, finalizedPeriod)
	require.NoError(t, err)
	require.True(t, cost3Rewards.IsZero())

	// costk 2 should earn double of costk 1
	coins.RequireCoinsDiffInPointOnePercentMargin(t, cost2Rewards, cost1Rewards.MulInt(sdkmath.NewInt(2)))

	totalClaimable := cost1Rewards.Add(cost2Rewards...)
	coins.RequireCoinsDiffInPointOnePercentMargin(t, totalDeposited, totalClaimable)
}
