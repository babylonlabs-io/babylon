package keeper

import (
	"context"
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/coins"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func FuzzCheckFpSlashed(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2, del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

		// should not error even without any data in the trackers
		// because there is no delegation pair (fp, del1) to iterate
		err := k.FpSlashed(ctx, fp1)
		require.NoError(t, err)

		// for fp1 30% for del1 and 70% for del 2
		del1Fp1Percentage := uint64(30)
		del2Fp1Percentage := uint64(70)
		err = k.BtcDelegationActivated(ctx, fp1, del1, sdkmath.NewIntFromUint64(del1Fp1Percentage))
		require.NoError(t, err)
		err = k.BtcDelegationActivated(ctx, fp1, del2, sdkmath.NewIntFromUint64(del2Fp1Percentage))
		require.NoError(t, err)

		// for fp2 50/50% for each del
		eachDelFp2Percentage := uint64(50)
		err = k.BtcDelegationActivated(ctx, fp2, del1, sdkmath.NewIntFromUint64(eachDelFp2Percentage))
		require.NoError(t, err)
		err = k.BtcDelegationActivated(ctx, fp2, del2, sdkmath.NewIntFromUint64(eachDelFp2Percentage))
		require.NoError(t, err)

		rwdFp1 := datagen.GenRandomCoins(r)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp1, rwdFp1)
		require.NoError(t, err)

		rwdFp2 := datagen.GenRandomCoins(r)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp2, rwdFp2)
		require.NoError(t, err)

		// slashes the fp1
		err = k.FpSlashed(ctx, fp1)
		require.NoError(t, err)

		del1Fp1Rwds := coins.CalculatePercentageOfCoins(rwdFp1, del1Fp1Percentage)
		del1RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del1)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, del1Fp1Rwds, del1RwdGauge.Coins)

		del2Fp1Rwds := coins.CalculatePercentageOfCoins(rwdFp1, del2Fp1Percentage)
		del2RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del2)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, del2Fp1Rwds, del2RwdGauge.Coins)

		// verifies that everything was deleted for fp1
		_, err = k.GetBTCDelegationRewardsTracker(ctx, fp1, del1)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())
		_, err = k.GetBTCDelegationRewardsTracker(ctx, fp1, del2)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())
		_, err = k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())
		_, err = k.GetFinalityProviderHistoricalRewards(ctx, fp1, 1)
		require.EqualError(t, err, types.ErrFPHistoricalRewardsNotFound.Error())

		// verifies that for fp2 is all there
		_, err = k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.NoError(t, err)
		_, err = k.GetFinalityProviderHistoricalRewards(ctx, fp2, 1)
		require.NoError(t, err)

		count := 0
		err = k.iterBtcDelegationsByDelegator(ctx, del1, func(del, fp sdk.AccAddress) error {
			count++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, count, 1)

		count = 0
		err = k.iterBtcDelegationsByDelegator(ctx, del2, func(del, fp sdk.AccAddress) error {
			count++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, count, 1)

		// checks that nothing affected the rewards related to the other finality provider
		// that wasn't slashed.
		err = k.sendAllBtcRewardsToGauge(ctx, del1)
		require.NoError(t, err)

		fp2RwdForEachDel := coins.CalculatePercentageOfCoins(rwdFp2, eachDelFp2Percentage)

		lastDel1RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del1)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, del1Fp1Rwds.Add(fp2RwdForEachDel...), lastDel1RwdGauge.Coins)

		err = k.sendAllBtcRewardsToGauge(ctx, del2)
		require.NoError(t, err)

		lastDel2RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del2)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, del2Fp1Rwds.Add(fp2RwdForEachDel...), lastDel2RwdGauge.Coins)
	})
}

func FuzzCheckSendAllBtcRewardsToGauge(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2, del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

		// should not error even without any data in the trackers
		// because there is no delegation pair (fp, del1) to iterate
		err := k.sendAllBtcRewardsToGauge(ctx, del1)
		require.NoError(t, err)

		// for fp1 30% for del1 and 70% for del 2
		del1Fp1Percentage := uint64(30)
		del2Fp1Percentage := uint64(70)
		err = k.BtcDelegationActivated(ctx, fp1, del1, sdkmath.NewIntFromUint64(del1Fp1Percentage))
		require.NoError(t, err)
		err = k.BtcDelegationActivated(ctx, fp1, del2, sdkmath.NewIntFromUint64(del2Fp1Percentage))
		require.NoError(t, err)

		// for fp2 50/50% for each del
		eachDelFp2Percentage := uint64(50)
		err = k.BtcDelegationActivated(ctx, fp2, del1, sdkmath.NewIntFromUint64(eachDelFp2Percentage))
		require.NoError(t, err)
		err = k.BtcDelegationActivated(ctx, fp2, del2, sdkmath.NewIntFromUint64(eachDelFp2Percentage))
		require.NoError(t, err)

		rwdFp1 := datagen.GenRandomCoins(r)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp1, rwdFp1)
		require.NoError(t, err)

		rwdFp2 := datagen.GenRandomCoins(r)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp2, rwdFp2)
		require.NoError(t, err)

		// calculates rewards for the del1 first
		err = k.sendAllBtcRewardsToGauge(ctx, del1)
		require.NoError(t, err)

		del1Fp1Rwds := coins.CalculatePercentageOfCoins(rwdFp1, del1Fp1Percentage)
		fp2RwdForEachDel := coins.CalculatePercentageOfCoins(rwdFp2, eachDelFp2Percentage)
		expectedRwdDel1 := del1Fp1Rwds.Add(fp2RwdForEachDel...)
		del1RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del1)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, expectedRwdDel1, del1RwdGauge.Coins)

		// calculates rewards for the del2
		err = k.sendAllBtcRewardsToGauge(ctx, del2)
		require.NoError(t, err)

		del2Fp1Rwds := coins.CalculatePercentageOfCoins(rwdFp1, del2Fp1Percentage)
		expectedRwdDel2 := del2Fp1Rwds.Add(fp2RwdForEachDel...)
		del2RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del2)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, expectedRwdDel2, del2RwdGauge.Coins)

		// check if send all the rewards again something changes, it shouldn't
		err = k.sendAllBtcRewardsToGauge(ctx, del1)
		require.NoError(t, err)

		newDel1RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del1)
		require.Equal(t, newDel1RwdGauge.Coins.String(), del1RwdGauge.Coins.String())

		// sends new rewards for fp2 which is 50/50
		rwdFp2 = datagen.GenRandomCoins(r)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp2, rwdFp2)
		require.NoError(t, err)

		err = k.sendAllBtcRewardsToGauge(ctx, del1)
		require.NoError(t, err)

		lastFp2RwdForEachDel := coins.CalculatePercentageOfCoins(rwdFp2, eachDelFp2Percentage)
		lastDel1RwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del1)
		lastExpectedRwdDel1 := del1Fp1Rwds.Add(fp2RwdForEachDel...).Add(lastFp2RwdForEachDel...)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, lastExpectedRwdDel1, lastDel1RwdGauge.Coins)
		require.Equal(t, lastFp2RwdForEachDel.String(), lastDel1RwdGauge.Coins.Sub(newDel1RwdGauge.Coins...).String())
	})
}

func FuzzCheckBtcDelegationModifiedWithPreInitDel(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp, del := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		count := 0
		fCount := func(ctx context.Context, fp, del sdk.AccAddress) error {
			count++
			return nil
		}
		require.Equal(t, count, 0)

		err := k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, fCount)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())

		err = k.BtcDelegationActivated(ctx, fp, del, sdkmath.NewIntFromUint64(datagen.RandomInt(r, 1000)+10))
		require.NoError(t, err)

		delRwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del)
		require.Nil(t, delRwdGauge)

		coinsToDel := datagen.GenRandomCoins(r)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp, coinsToDel)
		require.NoError(t, err)

		err = k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, fCount)
		require.NoError(t, err)
		require.Equal(t, count, 2)

		delRwdGauge = k.GetRewardGauge(ctx, types.BTC_STAKER, del)
		coins.RequireCoinsDiffInPointOnePercentMargin(t, delRwdGauge.Coins, coinsToDel)
		// note: the difference here in one micro coin value is expected due to the loss of precision in the BTC reward tracking mechanism
		// that needs to keep track of how much rewards 1 satoshi is entitled to receive.
		// expected: "10538986AnIGK,10991059BQZFY,19858803DTFwK,18591052NPLYN,11732268RmOWl,17440819TMPYN,17161570WfgTh,17743833aMJHg,19321764evLoF,17692017eysTF,15763155fbRbV,15691503jtkIM,15782745onTeQ,19076817pycDX,10059521tfcfY,13053824tgYdv,16164439ufZLL,15295587xvzKC"
		// actual  : "10538987AnIGK,10991060BQZFY,19858804DTFwK,18591053NPLYN,11732269RmOWl,17440820TMPYN,17161571WfgTh,17743834aMJHg,19321765evLoF,17692018eysTF,15763156fbRbV,15691504jtkIM,15782746onTeQ,19076818pycDX,10059522tfcfY,13053825tgYdv,16164440ufZLL,15295588xvzKC"
		// require.Equal(t, delRwdGauge.Coins.String(), coinsToDel.String())
	})
}

func FuzzCheckCalculateBTCDelegationRewardsAndSendToGauge(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp, del := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		btcRwd := datagen.GenRandomBTCDelegationRewardsTracker(r)
		err := k.setBTCDelegationRewardsTracker(ctx, fp, del, btcRwd)
		require.NoError(t, err)

		startHist, endHist := datagen.GenRandomFPHistRwdStartAndEnd(r)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, btcRwd.StartPeriodCumulativeReward, startHist)
		require.NoError(t, err)
		endPeriod := btcRwd.StartPeriodCumulativeReward + datagen.RandomInt(r, 10) + 1
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, endPeriod, endHist)
		require.NoError(t, err)

		expectedRwd := endHist.CumulativeRewardsPerSat.Sub(startHist.CumulativeRewardsPerSat...)
		expectedRwd = expectedRwd.MulInt(btcRwd.TotalActiveSat)
		expectedRwd = expectedRwd.QuoInt(types.DecimalRewards)

		rwdGauge := datagen.GenRandomRewardGauge(r)
		k.SetRewardGauge(ctx, types.BTC_STAKER, del, rwdGauge)

		err = k.CalculateBTCDelegationRewardsAndSendToGauge(ctx, fp, del, endPeriod)
		require.NoError(t, err)

		delRwdGauge := k.GetRewardGauge(ctx, types.BTC_STAKER, del)
		require.Equal(t, rwdGauge.Coins.Add(expectedRwd...).String(), delRwdGauge.Coins.String())
	})
}

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
		btcRwd = datagen.GenRandomBTCDelegationRewardsTracker(r)
		err = k.setBTCDelegationRewardsTracker(ctx, fp, del, btcRwd)
		require.NoError(t, err)

		startHist, endHist := datagen.GenRandomFPHistRwdStartAndEnd(r)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, btcRwd.StartPeriodCumulativeReward, startHist)
		require.NoError(t, err)
		endPeriod := btcRwd.StartPeriodCumulativeReward + datagen.RandomInt(r, 10) + 1
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, endPeriod, endHist)
		require.NoError(t, err)

		expectedRwd := endHist.CumulativeRewardsPerSat.Sub(startHist.CumulativeRewardsPerSat...)
		expectedRwd = expectedRwd.MulInt(btcRwd.TotalActiveSat)
		expectedRwd = expectedRwd.QuoInt(types.DecimalRewards)

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
		fp := datagen.GenRandomAddress()

		btcRwd := datagen.GenRandomBTCDelegationRewardsTracker(r)
		badEndedPeriod := btcRwd.StartPeriodCumulativeReward - 1
		require.Panics(t, func() {
			_, _ = k.calculateDelegationRewardsBetween(ctx, fp, btcRwd, badEndedPeriod)
		})

		historicalStartPeriod := datagen.GenRandomFPHistRwd(r)
		historicalStartPeriod.CumulativeRewardsPerSat = historicalStartPeriod.CumulativeRewardsPerSat.MulInt(types.DecimalRewards)
		err := k.setFinalityProviderHistoricalRewards(ctx, fp, btcRwd.StartPeriodCumulativeReward, historicalStartPeriod)
		require.NoError(t, err)

		endingPeriod := btcRwd.StartPeriodCumulativeReward + 1

		// creates a bad historical ending period that has less rewards than the starting one
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, endingPeriod, types.NewFinalityProviderHistoricalRewards(historicalStartPeriod.CumulativeRewardsPerSat.QuoInt(math.NewInt(2)), uint32(1)))
		require.NoError(t, err)
		require.Panics(t, func() {
			_, _ = k.calculateDelegationRewardsBetween(ctx, fp, btcRwd, endingPeriod)
		})

		// creates a correct historical rewards that has more rewards than the historical
		historicalEndingPeriod := datagen.GenRandomFPHistRwd(r)
		historicalEndingPeriod.CumulativeRewardsPerSat = historicalEndingPeriod.CumulativeRewardsPerSat.MulInt(types.DecimalRewards)
		historicalEndingPeriod.CumulativeRewardsPerSat = historicalEndingPeriod.CumulativeRewardsPerSat.Add(historicalStartPeriod.CumulativeRewardsPerSat...)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, endingPeriod, historicalEndingPeriod)
		require.NoError(t, err)

		expectedRewards := historicalEndingPeriod.CumulativeRewardsPerSat.Sub(historicalStartPeriod.CumulativeRewardsPerSat...)
		expectedRewards = expectedRewards.MulInt(btcRwd.TotalActiveSat)
		expectedRewards = expectedRewards.QuoInt(types.DecimalRewards)

		delRewards, err := k.calculateDelegationRewardsBetween(ctx, fp, btcRwd, endingPeriod)
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
		err = k.SetFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
		require.NoError(t, err)

		amtRwdInHistorical := fpCurrentRwd.CurrentRewards.MulInt(types.DecimalRewards).QuoInt(math.NewInt(2))
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period-1, types.NewFinalityProviderHistoricalRewards(amtRwdInHistorical, uint32(1)))
		require.NoError(t, err)

		endedPeriod, err = k.IncrementFinalityProviderPeriod(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, endedPeriod, fpCurrentRwd.Period)

		historicalEndedPeriod, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, endedPeriod)
		require.NoError(t, err)

		expectedHistoricalRwd := amtRwdInHistorical.Add(fpCurrentRwd.CurrentRewards.MulInt(types.DecimalRewards).QuoInt(fpCurrentRwd.TotalActiveSat)...)
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
		err = k.SetFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
		require.NoError(t, err)

		err = k.initializeBTCDelegation(ctx, fp, del)
		require.EqualError(t, err, types.ErrFPHistoricalRewardsNotFound.Error())

		fpHistoricalRwd := datagen.GenRandomFPHistRwd(r)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period-1, fpHistoricalRwd)
		require.NoError(t, err)

		err = k.initializeBTCDelegation(ctx, fp, del)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())

		delBtcRwdTrackerBeforeInitialize := datagen.GenRandomBTCDelegationRewardsTracker(r)
		err = k.setBTCDelegationRewardsTracker(ctx, fp, del, delBtcRwdTrackerBeforeInitialize)
		require.NoError(t, err)

		// increment period since reference count is already increased in the previous
		// initializeBTCDelegation
		endedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, endedPeriod, fpCurrentRwd.Period)

		err = k.initializeBTCDelegation(ctx, fp, del)
		require.NoError(t, err)

		actBtcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
		require.NoError(t, err)
		require.Equal(t, fpCurrentRwd.Period, actBtcDelRwdTracker.StartPeriodCumulativeReward)
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
	checkFpHistoricalRwd(t, ctx, k, fp1, fp1EndedPeriod, newBaseCoins(4000).MulInt(types.DecimalRewards))
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
