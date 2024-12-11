package keeper

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func FuzzCheckCalculateBTCDelegationRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp, del := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		btcRwd := datagen.GenRandomBTCDelegationRewardsTracker(r)

		rwd, err := k.CalculateBTCDelegationRewards(ctx, fp, del, btcRwd.StartPeriodCumulativeReward)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())
		require.Equal(t, rwd, sdk.Coins{})

		btcRwd.TotalActiveSat = math.ZeroInt()
		err = k.setBTCDelegationRewardsTracker(ctx, fp, del, btcRwd)
		require.NoError(t, err)

		rwd, err = k.CalculateBTCDelegationRewards(ctx, fp, del, btcRwd.StartPeriodCumulativeReward)
		require.NoError(t, err)
		require.Equal(t, rwd, sdk.NewCoins())

		// sets a real one but with a bad period
		btcRwd = datagen.GenRandomBTCDelegationRewardsTracker(r)
		err = k.setBTCDelegationRewardsTracker(ctx, fp, del, btcRwd)
		require.NoError(t, err)

		badEndedPeriod := btcRwd.StartPeriodCumulativeReward - 1
		require.Panics(t, func() {
			_, _ = k.CalculateBTCDelegationRewards(ctx, fp, del, badEndedPeriod)
		})

		// Creates a correct and expected historical periods with start and ending properly set
		startHist, endHist := datagen.GenRandomFPHistRwdStartAndEnd(r)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, btcRwd.StartPeriodCumulativeReward, startHist)
		require.NoError(t, err)
		endPeriod := btcRwd.StartPeriodCumulativeReward + datagen.RandomInt(r, 10)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, endPeriod, endHist)
		require.NoError(t, err)

		expectedRwd := endHist.CumulativeRewardsPerSat.Sub(startHist.CumulativeRewardsPerSat...)
		expectedRwd = expectedRwd.MulInt(btcRwd.TotalActiveSat)
		expectedRwd = expectedRwd.QuoInt(types.DecimalAccumulatedRewards)

		rwd, err = k.CalculateBTCDelegationRewards(ctx, fp, del, endPeriod)
		require.NoError(t, err)
		require.Equal(t, rwd.String(), expectedRwd.String())
	})
}

func FuzzCheckCalculateDelegationRewardsBetween(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp, del := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		btcRwd := datagen.GenRandomBTCDelegationRewardsTracker(r)
		badEndedPeriod := btcRwd.StartPeriodCumulativeReward - 1
		require.Panics(t, func() {
			_, _ = k.calculateDelegationRewardsBetween(ctx, fp, del, btcRwd, badEndedPeriod)
		})

		historicalStartPeriod := datagen.GenRandomFPHistRwd(r)
		historicalStartPeriod.CumulativeRewardsPerSat = historicalStartPeriod.CumulativeRewardsPerSat.MulInt(types.DecimalAccumulatedRewards)
		err := k.setFinalityProviderHistoricalRewards(ctx, fp, btcRwd.StartPeriodCumulativeReward, historicalStartPeriod)
		require.NoError(t, err)

		endingPeriod := btcRwd.StartPeriodCumulativeReward + 1

		// creates a bad historical ending period that has less rewards than the starting one
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, endingPeriod, types.NewFinalityProviderHistoricalRewards(historicalStartPeriod.CumulativeRewardsPerSat.QuoInt(math.NewInt(2))))
		require.NoError(t, err)
		require.Panics(t, func() {
			_, _ = k.calculateDelegationRewardsBetween(ctx, fp, del, btcRwd, endingPeriod)
		})

		// creates a correct historical rewards that has more rewards than the historical
		historicalEndingPeriod := datagen.GenRandomFPHistRwd(r)
		historicalEndingPeriod.CumulativeRewardsPerSat = historicalEndingPeriod.CumulativeRewardsPerSat.MulInt(types.DecimalAccumulatedRewards)
		historicalEndingPeriod.CumulativeRewardsPerSat = historicalEndingPeriod.CumulativeRewardsPerSat.Add(historicalStartPeriod.CumulativeRewardsPerSat...)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, endingPeriod, historicalEndingPeriod)
		require.NoError(t, err)

		expectedRewards := historicalEndingPeriod.CumulativeRewardsPerSat.Sub(historicalStartPeriod.CumulativeRewardsPerSat...)
		expectedRewards = expectedRewards.MulInt(btcRwd.TotalActiveSat)
		expectedRewards = expectedRewards.QuoInt(types.DecimalAccumulatedRewards)

		delRewards, err := k.calculateDelegationRewardsBetween(ctx, fp, del, btcRwd, endingPeriod)
		require.NoError(t, err)
		require.Equal(t, expectedRewards.String(), delRewards.String())
	})
}

func FuzzCheckAddFinalityProviderRewardsForBtcDelegations(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp := datagen.GenRandomAddress()

		coinsAdded := datagen.GenRandomCoins(r)
		// add rewards without initiliaze should error out
		err := k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp, coinsAdded)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())

		_, err = k.initializeFinalityProvider(ctx, fp)
		require.NoError(t, err)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp, coinsAdded)
		require.NoError(t, err)

		currentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, coinsAdded.String(), currentRwd.CurrentRewards.String())

		// adds again the same amounts
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp, coinsAdded)
		require.NoError(t, err)

		currentRwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, coinsAdded.MulInt(math.NewInt(2)).String(), currentRwd.CurrentRewards.String())
	})
}

func FuzzCheckIncrementFinalityProviderPeriod(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp := datagen.GenRandomAddress()

		// increment without initializing the FP
		endedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp)
		require.NoError(t, err, types.ErrFPCurrentRewardsNotFound.Error())
		require.Equal(t, endedPeriod, uint64(1))

		fpCurrentRwd := datagen.GenRandomFinalityProviderCurrentRewards(r)
		err = k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
		require.NoError(t, err)

		amtRwdInHistorical := fpCurrentRwd.CurrentRewards.MulInt(types.DecimalAccumulatedRewards).QuoInt(math.NewInt(2))
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period-1, types.NewFinalityProviderHistoricalRewards(amtRwdInHistorical))
		require.NoError(t, err)

		endedPeriod, err = k.IncrementFinalityProviderPeriod(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, endedPeriod, fpCurrentRwd.Period)

		historicalEndedPeriod, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, endedPeriod)
		require.NoError(t, err)

		expectedHistoricalRwd := amtRwdInHistorical.Add(fpCurrentRwd.CurrentRewards.MulInt(types.DecimalAccumulatedRewards).QuoInt(fpCurrentRwd.TotalActiveSat)...)
		require.Equal(t, historicalEndedPeriod.CumulativeRewardsPerSat.String(), expectedHistoricalRwd.String())

		newFPCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, newFPCurrentRwd.CurrentRewards.String(), sdk.NewCoins().String())
		require.Equal(t, newFPCurrentRwd.Period, fpCurrentRwd.Period+1)
		require.Equal(t, newFPCurrentRwd.TotalActiveSat, fpCurrentRwd.TotalActiveSat)
	})
}

func FuzzCheckInitializeBTCDelegation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp, del := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		err := k.initializeBTCDelegation(ctx, fp, del)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())

		fpCurrentRwd := datagen.GenRandomFinalityProviderCurrentRewards(r)
		err = k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
		require.NoError(t, err)

		err = k.initializeBTCDelegation(ctx, fp, del)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())

		delBtcRwdTrackerBeforeInitialize := datagen.GenRandomBTCDelegationRewardsTracker(r)
		err = k.setBTCDelegationRewardsTracker(ctx, fp, del, delBtcRwdTrackerBeforeInitialize)
		require.NoError(t, err)

		err = k.initializeBTCDelegation(ctx, fp, del)
		require.NoError(t, err)

		actBtcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
		require.NoError(t, err)
		require.Equal(t, fpCurrentRwd.Period-1, actBtcDelRwdTracker.StartPeriodCumulativeReward)
		require.Equal(t, delBtcRwdTrackerBeforeInitialize.TotalActiveSat, actBtcDelRwdTracker.TotalActiveSat)
	})
}

func FuzzCheckInitializeFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()

		k, ctx := NewKeeperWithCtx(t)
		fp := datagen.GenRandomAddress()

		currentRwdFp, err := k.initializeFinalityProvider(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, currentRwdFp.CurrentRewards.String(), sdk.NewCoins().String())
		require.Equal(t, currentRwdFp.TotalActiveSat.String(), math.ZeroInt().String())
		require.Equal(t, currentRwdFp.Period, uint64(1))

		histRwdFp, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, 0)
		require.NoError(t, err)
		require.Equal(t, histRwdFp.CumulativeRewardsPerSat.String(), sdk.NewCoins().String())

		// if initializes it again, the values should be the same
		currentRwdFp, err = k.initializeFinalityProvider(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, currentRwdFp.CurrentRewards.String(), sdk.NewCoins().String())
		require.Equal(t, currentRwdFp.TotalActiveSat.String(), math.ZeroInt().String())
		require.Equal(t, currentRwdFp.Period, uint64(1))

		histRwdFp, err = k.GetFinalityProviderHistoricalRewards(ctx, fp, 0)
		require.NoError(t, err)
		require.Equal(t, histRwdFp.CumulativeRewardsPerSat.String(), sdk.NewCoins().String())
	})
}

func TestIncrementFinalityProviderPeriod(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	del1 := datagen.GenRandomAddress()

	fp1EndedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp1)
	require.NoError(t, err)
	require.Equal(t, fp1EndedPeriod, uint64(1))

	checkFpCurrentRwd(t, ctx, k, fp1, fp1EndedPeriod, sdk.NewCoins(), math.NewInt(0))
	checkFpHistoricalRwd(t, ctx, k, fp1, 0, sdk.NewCoins())

	rwdAddedToPeriod1 := newBaseCoins(2_000000) // 2bbn
	err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp1, rwdAddedToPeriod1)
	require.NoError(t, err)

	// historical should not modify the rewards for the period already created
	checkFpHistoricalRwd(t, ctx, k, fp1, 0, sdk.NewCoins())
	checkFpCurrentRwd(t, ctx, k, fp1, fp1EndedPeriod, rwdAddedToPeriod1, math.NewInt(0))

	// needs to add some voting power so it can calculate the amount of rewards per share
	satsDelegated := math.NewInt(500)
	err = k.addDelegationSat(ctx, fp1, del1, satsDelegated)
	require.NoError(t, err)

	fp1EndedPeriod, err = k.IncrementFinalityProviderPeriod(ctx, fp1)
	require.NoError(t, err)
	require.Equal(t, fp1EndedPeriod, uint64(1))

	// now the historical that just ended should have as cumulative rewards 4000ubbn 2_000000ubbn/500sats
	checkFpHistoricalRwd(t, ctx, k, fp1, fp1EndedPeriod, newBaseCoins(4000).MulInt(types.DecimalAccumulatedRewards))
	checkFpCurrentRwd(t, ctx, k, fp1, fp1EndedPeriod+1, sdk.NewCoins(), satsDelegated)

	fp2EndedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp2)
	require.NoError(t, err)
	require.Equal(t, fp2EndedPeriod, uint64(1))
}

func checkFpHistoricalRwd(t *testing.T, ctx sdk.Context, k *Keeper, fp sdk.AccAddress, period uint64, expectedRwd sdk.Coins) {
	historical, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, period)
	require.NoError(t, err)
	require.Equal(t, historical.CumulativeRewardsPerSat.String(), expectedRwd.String())
}

func checkFpCurrentRwd(t *testing.T, ctx sdk.Context, k *Keeper, fp sdk.AccAddress, expectedPeriod uint64, expectedRwd sdk.Coins, totalActiveSat math.Int) {
	fp1CurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	require.NoError(t, err)
	require.Equal(t, fp1CurrentRwd.CurrentRewards.String(), expectedRwd.String())
	require.Equal(t, fp1CurrentRwd.Period, expectedPeriod)
	require.Equal(t, fp1CurrentRwd.TotalActiveSat.String(), totalActiveSat.String())
}

func newBaseCoins(amt uint64) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewIntFromUint64(amt)))
}
