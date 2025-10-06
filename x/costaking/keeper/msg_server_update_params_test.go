package keeper

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
)

func TestUpdateAllCostakersScore_VulnerabilityNoMocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ictvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, ictvK)
	// Setup two costakers
	attacker := datagen.GenRandomAddress()
	victim := datagen.GenRandomAddress()
	initialRatio := sdkmath.NewInt(50)

	dp := types.DefaultParams()
	dp.ScoreRatioBtcByBaby = initialRatio
	err := k.SetParams(ctx, dp)
	require.NoError(t, err)

	// Attacker: 5000 sats, 50000 baby -> score = min(5000, 50000/50) = 1000
	activeSats1 := sdkmath.NewInt(5000)
	activeBaby1 := sdkmath.NewInt(50000)
	err = k.costakerModifiedActiveAmounts(ctx, attacker, activeSats1, activeBaby1)
	require.NoError(t, err)
	// Victim: 10000 sats, 100000 baby -> score = min(10000, 100000/50) = 2000
	activeSats2 := sdkmath.NewInt(10000)
	activeBaby2 := sdkmath.NewInt(100000)
	err = k.costakerModifiedActiveAmounts(ctx, victim, activeSats2, activeBaby2)
	require.NoError(t, err)
	// Verify total score: 3000
	currentRwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(3000), currentRwd.TotalScore)
	// Deposit 30,000 tokens as rewards
	totalDeposited := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(30000)))
	err = k.AddRewardsForCostakers(ctx, totalDeposited)
	require.NoError(t, err)
	// Finalize the period so rewards become claimable
	periodBeforeUpdate, err := k.IncrementRewardsPeriod(ctx)
	require.NoError(t, err)
	// BEFORE UPDATE: Calculate total claimable rewards
	attackerRewardsBefore, err := k.CalculateCostakerRewards(ctx, attacker, periodBeforeUpdate)
	require.NoError(t, err)
	victimRewardsBefore, err := k.CalculateCostakerRewards(ctx, victim, periodBeforeUpdate)
	require.NoError(t, err)
	totalClaimableBefore := attackerRewardsBefore.Add(victimRewardsBefore...)
	t.Logf("\n=== BEFORE GOVERNANCE UPDATE ===")
	t.Logf("Total deposited rewards: %s", totalDeposited)
	t.Logf("Attacker claimable: %s (score: 1000)", attackerRewardsBefore)
	t.Logf("Victim claimable: %s (score: 2000)", victimRewardsBefore)
	t.Logf("Total claimable: %s", totalClaimableBefore)
	t.Logf("Invariant check: %s <= %s ✓", totalClaimableBefore, totalDeposited)
	// Verify invariant BEFORE update: claimable <= deposited
	require.True(t, totalClaimableBefore.IsAllLTE(totalDeposited), "BEFORE update: total claimable (%s) should be <= deposited (%s)", totalClaimableBefore, totalDeposited)
	// GOVERNANCE UPDATES PARAMETER
	// This changes the ratio from 50 to 25, doubling scores for these costakers
	newRatio := sdkmath.NewInt(25)
	t.Logf("\n=== GOVERNANCE PARAMETER UPDATE ===")
	t.Logf("ScoreRatioBtcByBaby: 50 -> 25")
	err = k.UpdateAllCostakersScore(ctx, newRatio)
	require.NoError(t, err)
	// Verify scores doubled
	attackerTrackerAfter, err := k.GetCostakerRewards(ctx, attacker)
	require.NoError(t, err)
	victimTrackerAfter, err := k.GetCostakerRewards(ctx, victim)
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(2000), attackerTrackerAfter.TotalScore, "Attacker score should double")
	require.Equal(t, sdkmath.NewInt(4000), victimTrackerAfter.TotalScore, "Victim score should double")
	t.Logf("Attacker NEW score: %s (doubled from 1000)", attackerTrackerAfter.TotalScore)
	t.Logf("Victim NEW score: %s (doubled from 2000)", victimTrackerAfter.TotalScore)
	// CHECK: What happened to StartPeriodCumulativeReward?
	attackerTrackerBefore, err := k.GetCostakerRewards(ctx, attacker)
	require.NoError(t, err)
	victimTrackerBefore, err := k.GetCostakerRewards(ctx, victim)
	require.NoError(t, err)
	t.Logf("\n=== TRACKER STATE CHECK ===")
	t.Logf("Attacker StartPeriodCumulativeReward: %d", attackerTrackerBefore.StartPeriodCumulativeReward)
	t.Logf("Victim StartPeriodCumulativeReward: %d", victimTrackerBefore.StartPeriodCumulativeReward)
	t.Logf("Period that was finalized: %d", periodBeforeUpdate)
	// AFTER UPDATE: Calculate total claimable rewards
	// The key bug: we calculate from the ORIGINAL period (periodBeforeUpdate)
	// but with the NEW inflated scores!
	attackerRewardsAfter, err := k.CalculateCostakerRewards(ctx, attacker, periodBeforeUpdate)
	require.NoError(t, err)
	victimRewardsAfter, err := k.CalculateCostakerRewards(ctx, victim, periodBeforeUpdate)
	require.NoError(t, err)
	totalClaimableAfter := attackerRewardsAfter.Add(victimRewardsAfter...)
	t.Logf("\n=== AFTER GOVERNANCE UPDATE ===")
	t.Logf("Total deposited rewards: %s (UNCHANGED)", totalDeposited)
	t.Logf("Attacker claimable: %s (score: 2000)", attackerRewardsAfter)
	t.Logf("Victim claimable: %s (score: 4000)", victimRewardsAfter)
	t.Logf("Total claimable: %s", totalClaimableAfter)
	t.Logf("Invariant check: %s <= %s ✗ VIOLATED!", totalClaimableAfter, totalDeposited)
	// Verify invariant is broken after update
	require.False(t, totalClaimableAfter.IsAllLTE(totalDeposited), "BUG DEMONSTRATED: total claimable (%s) EXCEEDS deposited (%s)!", totalClaimableAfter, totalDeposited)
	require.Equal(t, "20000ubbn", attackerRewardsAfter.String(), "Attacker can claim 20000 (was 10000) - DOUBLED")
	require.Equal(t, "40000ubbn", victimRewardsAfter.String(), "Victim can claim 40000 (was 20000) - DOUBLED")
	require.Equal(t, "60000ubbn", totalClaimableAfter.String(), "Total claimable is 60000 but only 30000 was deposited!")
	// Calculate the excess
	excessClaimable := totalClaimableAfter.Sub(totalDeposited...)
	t.Logf("\n=== VULNERABILITY SUMMARY ===")
	t.Logf("Deposited: %s", totalDeposited)
	t.Logf("Claimable before: %s (invariant holds)", totalClaimableBefore)
	t.Logf("Claimable after: %s (invariant BROKEN)", totalClaimableAfter)
	t.Logf("EXCESS: %s (200%% of deposits!)", excessClaimable)
	t.Logf("")
	t.Logf("ROOT CAUSE: UpdateAllCostakersScore updates scores but does NOT")
	t.Logf("update StartPeriodCumulativeReward, allowing costakers to claim")
	t.Logf("historical rewards retroactively with their NEW inflated scores.")
	t.Logf("")
	t.Logf("IMPACT: First withdrawers drain the module, leaving it insolvent.")
	t.Logf("Remaining costakers cannot claim their legitimate rewards.")
}
