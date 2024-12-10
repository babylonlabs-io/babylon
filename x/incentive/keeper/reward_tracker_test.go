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
	checkFpHistoricalRwd(t, ctx, k, fp1, fp1EndedPeriod, newBaseCoins(4000).MulInt(DecimalAccumulatedRewards))
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
