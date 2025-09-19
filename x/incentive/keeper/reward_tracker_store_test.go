package keeper

import (
	"math/rand"
	"testing"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

func FuzzCheckBtcDelegationActivated(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2, del1 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

		amtActivateFp1Del1 := datagen.RandomInt(r, 10) + 5
		amtActivateFp2Del1 := datagen.RandomInt(r, 4) + 1
		amtActivateBoth := datagen.RandomInt(r, 7) + 3

		// delegates for both pairs (fp1, del1) (fp2, del1)
		err := k.BtcDelegationActivated(ctx, fp1, del1, sdkmath.NewIntFromUint64(amtActivateFp1Del1))
		require.NoError(t, err)
		err = k.BtcDelegationActivated(ctx, fp2, del1, sdkmath.NewIntFromUint64(amtActivateFp2Del1))
		require.NoError(t, err)

		// verifies the amounts
		fp1Del1RwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp1, del1)
		require.NoError(t, err)
		require.Equal(t, fp1Del1RwdTracker.TotalActiveSat.Uint64(), amtActivateFp1Del1)

		fp2Del1RwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp2, del1)
		require.NoError(t, err)
		require.Equal(t, fp2Del1RwdTracker.TotalActiveSat.Uint64(), amtActivateFp2Del1)

		// delegates for both pairs again
		err = k.BtcDelegationActivated(ctx, fp1, del1, sdkmath.NewIntFromUint64(amtActivateBoth))
		require.NoError(t, err)
		err = k.BtcDelegationActivated(ctx, fp2, del1, sdkmath.NewIntFromUint64(amtActivateBoth))
		require.NoError(t, err)

		// verifies the amounts
		fp1Del1RwdTracker, err = k.GetBTCDelegationRewardsTracker(ctx, fp1, del1)
		require.NoError(t, err)
		require.Equal(t, fp1Del1RwdTracker.TotalActiveSat.Uint64(), amtActivateFp1Del1+amtActivateBoth)

		fp2Del1RwdTracker, err = k.GetBTCDelegationRewardsTracker(ctx, fp2, del1)
		require.NoError(t, err)
		require.Equal(t, fp2Del1RwdTracker.TotalActiveSat.Uint64(), amtActivateFp2Del1+amtActivateBoth)
	})
}

func FuzzCheckBtcDelegationUnbonded(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2, del1 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

		amtToActivate := datagen.RandomInt(r, 10) + 5
		fp1Del1ToUnbond := datagen.RandomInt(r, int(amtToActivate)-1) + 1
		err := k.BtcDelegationUnbonded(ctx, fp1, del1, sdkmath.NewIntFromUint64(fp1Del1ToUnbond))
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())

		// delegates for both pairs (fp1, del1) (fp2, del1)
		err = k.BtcDelegationActivated(ctx, fp1, del1, sdkmath.NewIntFromUint64(amtToActivate))
		require.NoError(t, err)
		err = k.BtcDelegationActivated(ctx, fp2, del1, sdkmath.NewIntFromUint64(amtToActivate))
		require.NoError(t, err)

		// unbonds more than it has, should error
		err = k.BtcDelegationUnbonded(ctx, fp1, del1, sdkmath.NewIntFromUint64(amtToActivate+1))
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNegativeAmount.Error())

		// normally unbonds only part of it
		err = k.BtcDelegationUnbonded(ctx, fp1, del1, sdkmath.NewIntFromUint64(fp1Del1ToUnbond))
		require.NoError(t, err)

		fp1Del1RwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp1, del1)
		require.NoError(t, err)
		require.Equal(t, fp1Del1RwdTracker.TotalActiveSat.Uint64(), amtToActivate-fp1Del1ToUnbond)

		// unbonds all
		err = k.BtcDelegationUnbonded(ctx, fp2, del1, sdkmath.NewIntFromUint64(amtToActivate))
		require.NoError(t, err)

		fp2Del1RwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp2, del1)
		require.NoError(t, err)
		require.True(t, fp2Del1RwdTracker.TotalActiveSat.IsZero())
	})
}

func FuzzCheckBTCDelegatorToFP(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
		del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		// only one set
		// del1 -> fp1
		k.setBTCDelegatorToFP(ctx, del1, fp1)
		count := 0
		err := k.iterBtcDelegationsByDelegator(ctx, del1, func(del, fp sdk.AccAddress) error {
			require.Equal(t, del.String(), del1.String())
			require.Equal(t, fp1.String(), fp.String())
			count++
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)

		// restart count every time
		// del1 -> fp1, fp2
		k.setBTCDelegatorToFP(ctx, del1, fp2)
		count = 0
		err = k.iterBtcDelegationsByDelegator(ctx, del1, func(del, fp sdk.AccAddress) error {
			count++
			require.Equal(t, del.String(), del1.String())
			if fp.Equals(fp1) {
				require.Equal(t, fp1.String(), fp.String())
				return nil
			}

			require.Equal(t, fp2.String(), fp.String())
			return nil
		})
		require.Equal(t, 2, count)
		require.NoError(t, err)

		// new delegator
		// del2 -> fp2
		k.setBTCDelegatorToFP(ctx, del2, fp2)
		count = 0
		err = k.iterBtcDelegationsByDelegator(ctx, del2, func(del, fp sdk.AccAddress) error {
			count++
			require.Equal(t, del.String(), del2.String())
			require.Equal(t, fp2.String(), fp.String())
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)

		// deletes del1 -> fp1
		// iterates again should only have the del1 -> fp2
		count = 0
		k.deleteBTCDelegatorToFP(ctx, del1, fp1)
		err = k.iterBtcDelegationsByDelegator(ctx, del1, func(del, fp sdk.AccAddress) error {
			require.Equal(t, del.String(), del1.String())
			require.Equal(t, fp2.String(), fp.String())
			count++
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)
	})
}

func FuzzCheckBTCDelegationRewardsTracker(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
		del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		// fp1, del1
		err := k.setBTCDelegationRewardsTracker(ctx, fp1, del1, types.NewBTCDelegationRewardsTracker(0, math.NewInt(100)))
		require.NoError(t, err)

		count := 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp1, func(fp, del sdk.AccAddress, val types.BTCDelegationRewardsTracker) error {
			count++
			require.Equal(t, fp1, fp)
			require.Equal(t, del1, del)
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)

		// fp1, del2
		err = k.setBTCDelegationRewardsTracker(ctx, fp1, del2, types.NewBTCDelegationRewardsTracker(0, math.NewInt(100)))
		require.NoError(t, err)

		count = 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp1, func(fp, del sdk.AccAddress, val types.BTCDelegationRewardsTracker) error {
			count++
			require.Equal(t, fp1, fp)
			if del1.Equals(del) {
				require.Equal(t, del1, del)
				return nil
			}
			require.Equal(t, del2, del)
			return nil
		})
		require.Equal(t, 2, count)
		require.NoError(t, err)

		// fp2, del1
		amtFp2Del1 := datagen.RandomMathInt(r, 20000)
		startPeriodFp2Del1 := datagen.RandomInt(r, 200)
		err = k.setBTCDelegationRewardsTracker(ctx, fp2, del1, types.NewBTCDelegationRewardsTracker(startPeriodFp2Del1, amtFp2Del1))
		require.NoError(t, err)

		btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp2, del1)
		require.NoError(t, err)
		require.Equal(t, amtFp2Del1.String(), btcDelRwdTracker.TotalActiveSat.String())
		require.Equal(t, startPeriodFp2Del1, btcDelRwdTracker.StartPeriodCumulativeReward)

		count = 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp2, func(fp, del sdk.AccAddress, val types.BTCDelegationRewardsTracker) error {
			count++
			require.Equal(t, fp2, fp)
			require.Equal(t, del1, del)
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)

		// check delete fp2
		k.deleteKeysFromBTCDelegationRewardsTracker(ctx, fp2, [][]byte{del1.Bytes()})
		count = 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp2, func(fp, del sdk.AccAddress, val types.BTCDelegationRewardsTracker) error {
			count++
			return nil
		})
		require.Equal(t, 0, count)
		require.NoError(t, err)

		// check delete all from fp1
		k.deleteKeysFromBTCDelegationRewardsTracker(ctx, fp1, [][]byte{del1.Bytes(), del2.Bytes()})
		count = 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp1, func(fp, del sdk.AccAddress, rwdTracker types.BTCDelegationRewardsTracker) error {
			count++
			return nil
		})
		require.Equal(t, 0, count)
		require.NoError(t, err)

		_, err = k.GetBTCDelegationRewardsTracker(ctx, fp2, del1)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())
	})
}

func FuzzCheckFinalityProviderCurrentRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		_, err := k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())

		expectedCurrentRwdFp1 := datagen.GenRandomFinalityProviderCurrentRewards(r)
		err = k.setFinalityProviderCurrentRewards(ctx, fp1, expectedCurrentRwdFp1)
		require.NoError(t, err)

		currentRwdFp1, err := k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.NoError(t, err)
		require.Equal(t, expectedCurrentRwdFp1.CurrentRewards.String(), currentRwdFp1.CurrentRewards.String())
		require.Equal(t, expectedCurrentRwdFp1.TotalActiveSat.String(), currentRwdFp1.TotalActiveSat.String())
		require.Equal(t, expectedCurrentRwdFp1.Period, currentRwdFp1.Period)

		k.deleteAllFromFinalityProviderRwd(ctx, fp1)
		_, err = k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())

		// sets a new fp
		err = k.setFinalityProviderCurrentRewards(ctx, fp2, datagen.GenRandomFinalityProviderCurrentRewards(r))
		require.NoError(t, err)

		_, err = k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.NoError(t, err)

		k.deleteFinalityProviderCurrentRewards(ctx, fp2)
		_, err = k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())
	})
}

func FuzzCheckFinalityProviderHistoricalRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		fp1Period1 := datagen.RandomInt(r, 10)
		_, err := k.GetFinalityProviderHistoricalRewards(ctx, fp1, fp1Period1)
		require.EqualError(t, err, types.ErrFPHistoricalRewardsNotFound.Error())

		expectedHistRwdFp1 := datagen.GenRandomFPHistRwd(r)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp1, fp1Period1, expectedHistRwdFp1)
		require.NoError(t, err)

		fp1Period1Historical, err := k.GetFinalityProviderHistoricalRewards(ctx, fp1, fp1Period1)
		require.NoError(t, err)
		require.Equal(t, expectedHistRwdFp1.CumulativeRewardsPerSat.String(), fp1Period1Historical.CumulativeRewardsPerSat.String())

		// sets multiple historical for fp2
		fp2Period1Historical := datagen.RandomInt(r, 10)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp2, fp2Period1Historical, datagen.GenRandomFPHistRwd(r))
		require.NoError(t, err)
		fp2Period2Historical := datagen.RandomInt(r, 10)
		err = k.setFinalityProviderHistoricalRewards(ctx, fp2, fp2Period2Historical, datagen.GenRandomFPHistRwd(r))
		require.NoError(t, err)

		// sets a new current fp rwd to check the delete all
		err = k.setFinalityProviderCurrentRewards(ctx, fp2, datagen.GenRandomFinalityProviderCurrentRewards(r))
		require.NoError(t, err)

		_, err = k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.NoError(t, err)

		// deleted all from fp2
		k.deleteAllFromFinalityProviderRwd(ctx, fp2)

		_, err = k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())

		_, err = k.GetFinalityProviderHistoricalRewards(ctx, fp2, fp2Period1Historical)
		require.EqualError(t, err, types.ErrFPHistoricalRewardsNotFound.Error())

		_, err = k.GetFinalityProviderHistoricalRewards(ctx, fp2, fp2Period2Historical)
		require.EqualError(t, err, types.ErrFPHistoricalRewardsNotFound.Error())
	})
}

func FuzzCheckSubFinalityProviderStaked(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		amtSub := datagen.RandomMathInt(r, 100)
		err := k.subFinalityProviderStaked(ctx, fp1, amtSub)
		require.EqualError(t, err, types.ErrFPCurrentRewardsNotFound.Error())

		fp2Set := datagen.GenRandomFinalityProviderCurrentRewards(r)
		err = k.setFinalityProviderCurrentRewards(ctx, fp2, fp2Set)
		require.NoError(t, err)

		err = k.subFinalityProviderStaked(ctx, fp2, fp2Set.TotalActiveSat)
		require.NoError(t, err)

		fp2CurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.NoError(t, err)
		require.True(t, fp2CurrentRwd.TotalActiveSat.IsZero())

		// subTotalActiveSat returns negative value - should fail
		err = k.subFinalityProviderStaked(ctx, fp2, math.NewInt(1000))
		require.Error(t, err)
		require.True(t, errorsmod.IsOf(err, types.ErrFPCurrentRewardsTrackerNegativeAmount))
	})
}

func FuzzCheckSubDelegationSat(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp, del := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		amtToSub := datagen.RandomMathInt(r, 10000).AddRaw(10)
		err := k.subDelegationSat(ctx, fp, del, amtToSub)
		require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())

		amtInRwd := amtToSub.AddRaw(120)
		err = k.setBTCDelegationRewardsTracker(ctx, fp, del, types.NewBTCDelegationRewardsTracker(1, amtInRwd))
		require.NoError(t, err)

		fpCurrentRwd := datagen.GenRandomFinalityProviderCurrentRewards(r)
		fpCurrentRwd.TotalActiveSat = amtInRwd
		err = k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
		require.NoError(t, err)

		err = k.subDelegationSat(ctx, fp, del, amtToSub)
		require.NoError(t, err)

		expectedAmt := amtInRwd.Sub(amtToSub)

		delRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
		require.NoError(t, err)
		require.Equal(t, expectedAmt.String(), delRwdTracker.TotalActiveSat.String())

		fpCurrentRwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp)
		require.NoError(t, err)
		require.Equal(t, expectedAmt.String(), fpCurrentRwd.TotalActiveSat.String())
	})
}

func FuzzCheckAddFinalityProviderStaked(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		amtAdded := datagen.RandomMathInt(r, 1000)
		err := k.addFinalityProviderStaked(ctx, fp1, amtAdded)
		require.NoError(t, err)

		currentRwdFp1, err := k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.NoError(t, err)
		require.Equal(t, currentRwdFp1.TotalActiveSat, amtAdded)
		require.Equal(t, currentRwdFp1.Period, uint64(1))
		require.Equal(t, currentRwdFp1.CurrentRewards.String(), sdk.NewCoins().String())

		err = k.addFinalityProviderStaked(ctx, fp1, amtAdded)
		require.NoError(t, err)
		currentRwdFp1, err = k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.NoError(t, err)
		require.Equal(t, currentRwdFp1.TotalActiveSat, amtAdded.MulRaw(2))
		require.Equal(t, currentRwdFp1.Period, uint64(1))
		require.Equal(t, currentRwdFp1.CurrentRewards.String(), sdk.NewCoins().String())

		currentRwdFp2, err := k.initializeFinalityProvider(ctx, fp2)
		require.NoError(t, err)

		rwdOnFp2 := datagen.GenRandomCoins(r)
		err = k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp2, rwdOnFp2)
		require.NoError(t, err)

		require.Equal(t, currentRwdFp2.TotalActiveSat, math.ZeroInt())
		require.Equal(t, currentRwdFp2.Period, uint64(1))
		require.Equal(t, currentRwdFp2.CurrentRewards.String(), sdk.NewCoins().String())

		amtAddedToFp2 := datagen.RandomMathInt(r, 1000)
		err = k.addFinalityProviderStaked(ctx, fp2, amtAddedToFp2)
		require.NoError(t, err)

		currentRwdFp2, err = k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.NoError(t, err)
		require.Equal(t, currentRwdFp2.TotalActiveSat.String(), amtAddedToFp2.String())
		require.Equal(t, currentRwdFp2.Period, uint64(1))
		require.Equal(t, currentRwdFp2.CurrentRewards.String(), rwdOnFp2.String())
	})
}

func FuzzCheckAddDelegationSat(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, del := datagen.GenRandomAddress(), datagen.GenRandomAddress()
		fp2 := datagen.GenRandomAddress()

		amtAdded := datagen.RandomMathInt(r, 1000)
		err := k.addDelegationSat(ctx, fp1, del, amtAdded)
		require.NoError(t, err)

		rwdTrackerFp1Del1, err := k.GetBTCDelegationRewardsTracker(ctx, fp1, del)
		require.NoError(t, err)
		require.Equal(t, amtAdded.String(), rwdTrackerFp1Del1.TotalActiveSat.String())
		require.Equal(t, uint64(0), rwdTrackerFp1Del1.StartPeriodCumulativeReward)

		currentRwdFp1, err := k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.NoError(t, err)
		require.Equal(t, amtAdded.String(), currentRwdFp1.TotalActiveSat.String())
		require.Equal(t, uint64(1), currentRwdFp1.Period)
		require.Equal(t, sdk.NewCoins().String(), currentRwdFp1.CurrentRewards.String())

		currentHistRwdFp1Del1, err := k.GetFinalityProviderHistoricalRewards(ctx, fp1, 0)
		require.NoError(t, err)
		require.Equal(t, sdk.NewCoins().String(), currentHistRwdFp1Del1.CumulativeRewardsPerSat.String())

		// add delegation again
		err = k.addDelegationSat(ctx, fp1, del, amtAdded)
		require.NoError(t, err)

		// just verifies that the amount duplicated, without modifying the periods
		rwdTrackerFp1Del1, err = k.GetBTCDelegationRewardsTracker(ctx, fp1, del)
		require.NoError(t, err)
		require.Equal(t, amtAdded.MulRaw(2).String(), rwdTrackerFp1Del1.TotalActiveSat.String())
		require.Equal(t, uint64(0), rwdTrackerFp1Del1.StartPeriodCumulativeReward)

		currentRwdFp1, err = k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.NoError(t, err)
		require.Equal(t, amtAdded.MulRaw(2).String(), currentRwdFp1.TotalActiveSat.String())
		require.Equal(t, uint64(1), currentRwdFp1.Period)
		require.Equal(t, sdk.NewCoins().String(), currentRwdFp1.CurrentRewards.String())

		currentHistRwdFp1Del1, err = k.GetFinalityProviderHistoricalRewards(ctx, fp1, 0)
		require.NoError(t, err)
		require.Equal(t, sdk.NewCoins().String(), currentHistRwdFp1Del1.CumulativeRewardsPerSat.String())

		// adds delegation sat to already initilialized FP and delegation
		// needs to initialize the FP first, then the delegation
		fp2CurrentRwd, err := k.initializeFinalityProvider(ctx, fp2)
		require.NoError(t, err)

		startingActiveAmt := datagen.RandomMathInt(r, 100)
		err = k.setBTCDelegationRewardsTracker(ctx, fp2, del, types.NewBTCDelegationRewardsTracker(fp2CurrentRwd.Period, startingActiveAmt))
		require.NoError(t, err)

		err = k.initializeBTCDelegation(ctx, fp2, del)
		require.NoError(t, err)

		err = k.addDelegationSat(ctx, fp2, del, amtAdded)
		require.NoError(t, err)

		// verifies the amount added
		rwdTrackerFp2Del1, err := k.GetBTCDelegationRewardsTracker(ctx, fp2, del)
		require.NoError(t, err)
		require.Equal(t, amtAdded.Add(startingActiveAmt).String(), rwdTrackerFp2Del1.TotalActiveSat.String())
		require.Equal(t, uint64(0), rwdTrackerFp2Del1.StartPeriodCumulativeReward)

		currentRwdFp2, err := k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.NoError(t, err)
		// since it was artificially set the starting amount of the delegation
		// the FP should only have the amount added.
		require.Equal(t, amtAdded.String(), currentRwdFp2.TotalActiveSat.String())
		require.Equal(t, uint64(1), currentRwdFp2.Period)
		require.Equal(t, sdk.NewCoins().String(), currentRwdFp2.CurrentRewards.String())

		currentHistRwdFp2Del1, err := k.GetFinalityProviderHistoricalRewards(ctx, fp2, 0)
		require.NoError(t, err)
		require.Equal(t, sdk.NewCoins().String(), currentHistRwdFp2Del1.CumulativeRewardsPerSat.String())
	})
}

func TestAddSubDelegationSat(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	fp1, del1 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	fp2, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	amtFp1Del1, amtFp1Del2, amtFp2Del2, amtFp2Del1 := math.NewInt(2000), math.NewInt(4000), math.NewInt(500), math.NewInt(700)

	_, err := k.GetBTCDelegationRewardsTracker(ctx, fp1, del1)
	require.EqualError(t, err, types.ErrBTCDelegationRewardsTrackerNotFound.Error())

	// adds 2000 for fp1, del1
	// fp1       => 2000
	// fp1, del1 => 2000
	err = k.addDelegationSat(ctx, fp1, del1, amtFp1Del1)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)

	btcDelRwdFp1Del1, err := k.GetBTCDelegationRewardsTracker(ctx, fp1, del1)
	require.NoError(t, err)
	// if the normal flow with initilize BTC delegation would have been called,
	// it would start as 1.
	require.Equal(t, btcDelRwdFp1Del1.StartPeriodCumulativeReward, uint64(0))

	// adds 4000 for fp1, del2
	// fp1       => 6000
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	err = k.addDelegationSat(ctx, fp1, del2, amtFp1Del2)
	require.NoError(t, err)

	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2))
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2)
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)

	// adds 500 for fp2, del2
	// fp1       => 6000
	// fp2       =>  500
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	// fp2, del2 =>  500
	err = k.addDelegationSat(ctx, fp2, del2, amtFp2Del2)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2)
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2)

	// adds 700 for fp2, del1
	// fp1       => 6000
	// fp2       => 1200
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	// fp2, del1 =>  700
	// fp2, del2 =>  500
	err = k.addDelegationSat(ctx, fp2, del1, amtFp2Del1)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2.Add(amtFp2Del1))
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2)
	checkFpDelTotalSat(t, ctx, k, fp2, del1, amtFp2Del1)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2)

	lastAmtFp1Del2 := math.NewInt(2000)
	// adds 2000 for fp1, del2
	// fp1       => 8000
	// fp2       => 1200
	// fp1, del1 => 2000
	// fp1, del2 => 6000
	// fp2, del1 =>  700
	// fp2, del2 =>  500
	err = k.addDelegationSat(ctx, fp1, del2, lastAmtFp1Del2)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2).Add(lastAmtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2.Add(amtFp2Del1))
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2.Add(lastAmtFp1Del2))
	checkFpDelTotalSat(t, ctx, k, fp2, del1, amtFp2Del1)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2)

	subAmtFp2Del2 := math.NewInt(350)
	// subtract 350 for fp2, del2
	// fp1       => 8000
	// fp2       =>  850
	// fp1, del1 => 2000
	// fp1, del2 => 6000
	// fp2, del1 =>  700
	// fp2, del2 =>  150
	err = k.subDelegationSat(ctx, fp2, del2, subAmtFp2Del2)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2).Add(lastAmtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2.Add(amtFp2Del1).Sub(subAmtFp2Del2))
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2.Add(lastAmtFp1Del2))
	checkFpDelTotalSat(t, ctx, k, fp2, del1, amtFp2Del1)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2.Sub(subAmtFp2Del2))
}

func checkFpTotalSat(t *testing.T, ctx sdk.Context, k *Keeper, fp sdk.AccAddress, expectedSat math.Int) {
	rwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	require.NoError(t, err)
	require.Equal(t, expectedSat.String(), rwd.TotalActiveSat.String())
}

func checkFpDelTotalSat(t *testing.T, ctx sdk.Context, k *Keeper, fp, del sdk.AccAddress, expectedSat math.Int) {
	rwd, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	require.NoError(t, err)
	require.Equal(t, expectedSat.String(), rwd.TotalActiveSat.String())
}

func TestIterateBTCDelegationSatsUpdated(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	del1, del2, del3 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := uint64(100)
	header := sdkCtx.HeaderInfo()
	header.Height = int64(currentHeight)
	ctx = sdkCtx.WithHeaderInfo(header)

	err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 50)
	require.NoError(t, err)

	// fp1 delegations:
	// (fp1, del1): 5000 sats
	// (fp1, del2): 3000 sats
	// (fp1, del3): 2000 sats
	err = k.addDelegationSat(ctx, fp1, del1, sdkmath.NewInt(5000))
	require.NoError(t, err)
	err = k.addDelegationSat(ctx, fp1, del2, sdkmath.NewInt(3000))
	require.NoError(t, err)
	err = k.addDelegationSat(ctx, fp1, del3, sdkmath.NewInt(2000))
	require.NoError(t, err)

	// fp2 delegations:
	// (fp2, del1): 4000 sats
	// (fp2, del2): 1500 sats
	err = k.addDelegationSat(ctx, fp2, del1, sdkmath.NewInt(4000))
	require.NoError(t, err)
	err = k.addDelegationSat(ctx, fp2, del2, sdkmath.NewInt(1500))
	require.NoError(t, err)

	// Block 55: (fp1, del1) add 1000
	err = k.AddEventBtcDelegationActivated(ctx, 55, fp1, del1, 1000)
	require.NoError(t, err)

	// Block 60: (fp1, del2) add 500
	err = k.AddEventBtcDelegationUnbonded(ctx, 60, fp1, del2, 500)
	require.NoError(t, err)

	// Block 65: (fp1, del3) add 800
	err = k.AddEventBtcDelegationActivated(ctx, 65, fp1, del3, 800)
	require.NoError(t, err)

	// Block 70: (fp2, del1) sub 1200
	err = k.AddEventBtcDelegationUnbonded(ctx, 70, fp2, del1, 1200)
	require.NoError(t, err)

	// Block 75: (fp2, del2) add 600
	err = k.AddEventBtcDelegationActivated(ctx, 75, fp2, del2, 600)
	require.NoError(t, err)

	// Block 80: (fp1, del1) sub 600
	err = k.AddEventBtcDelegationUnbonded(ctx, 80, fp1, del1, 300)
	require.NoError(t, err)

	// Block 85: (fp2, del3) add 1000
	err = k.AddEventBtcDelegationActivated(ctx, 85, fp2, del3, 1000)
	require.NoError(t, err)

	// Block 90: (fp2, del2) sub 200
	err = k.AddEventBtcDelegationUnbonded(ctx, 90, fp2, del2, 200)
	require.NoError(t, err)

	expectedFp1Results := map[string]int64{
		del1.String(): 5700, // 5000 + (1000-300) = 5700
		del2.String(): 2500, // 3000 - 500 = 2500
		del3.String(): 2800, // 2000 + 800 = 2800
	}

	actualFp1Results := make(map[string]int64)
	err = k.IterateBTCDelegationSatsUpdated(ctx, fp1, func(del sdk.AccAddress, activeSats sdkmath.Int) error {
		actualFp1Results[del.String()] = activeSats.Int64()
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, expectedFp1Results, actualFp1Results, "fp1 delegation amounts should match expected values")

	expectedFp2Results := map[string]int64{
		del1.String(): 2800, // 4000 - 1200 = 2800
		del2.String(): 1900, // 1500 + (600-200) = 1900
		del3.String(): 1000, // new delegation from events
	}

	actualFp2Results := make(map[string]int64)
	err = k.IterateBTCDelegationSatsUpdated(ctx, fp2, func(del sdk.AccAddress, activeSats sdkmath.Int) error {
		actualFp2Results[del.String()] = activeSats.Int64()
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, expectedFp2Results, actualFp2Results, "fp2 delegation amounts should match expected values")

	// Check delegations count
	fp1Count := 0
	err = k.IterateBTCDelegationSatsUpdated(ctx, fp1, func(del sdk.AccAddress, activeSats sdkmath.Int) error {
		fp1Count++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, fp1Count, "fp1 should have exactly 3 delegations")

	fp2Count := 0
	err = k.IterateBTCDelegationSatsUpdated(ctx, fp2, func(del sdk.AccAddress, activeSats sdkmath.Int) error {
		fp2Count++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, fp2Count, "fp2 should have exactly 3 delegations")

	// Check error handling
	expectedErr := errorsmod.Wrap(types.ErrBTCDelegationRewardsTrackerNotFound, "test error")
	err = k.IterateBTCDelegationSatsUpdated(ctx, fp1, func(del sdk.AccAddress, activeSats sdkmath.Int) error {
		return expectedErr
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "test error")

	// Check edge case - no stored data should result in empty iteration
	fp3 := datagen.GenRandomAddress()
	fp3Count := 0
	err = k.IterateBTCDelegationSatsUpdated(ctx, fp3, func(del sdk.AccAddress, activeSats sdkmath.Int) error {
		fp3Count++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, fp3Count, "fp3 with no stored data should have 0 delegations")
}

func NewKeeperWithCtx(t *testing.T) (*Keeper, sdk.Context) {
	encConf := appparams.DefaultEncodingConfig()
	ctx, kvStore := store.NewStoreWithCtx(t, types.ModuleName)
	k := NewKeeper(encConf.Codec, kvStore, nil, nil, nil, appparams.AccGov.String(), appparams.AccFeeCollector.String())
	return &k, ctx
}
