package keeper

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/stretchr/testify/require"
)

func FuzzProcessRewardTrackerEventsAtHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2, del1 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()
		blkHeight := datagen.RandomInt(r, 1000) + 2

		lastProcessedHeight, err := k.GetRewardTrackerEventLastProcessedHeight(ctx)
		require.NoError(t, err)
		require.EqualValues(t, lastProcessedHeight, 0)

		err = k.ProcessRewardTrackerEventsAtHeight(ctx, blkHeight)
		require.NoError(t, err)

		rAmtSat, rAmtSat2 := datagen.RandomInt(r, 1000)+1, datagen.RandomInt(r, 2000)+2

		err = k.AddEventBtcDelegationActivated(ctx, blkHeight, fp1, del1, rAmtSat)
		require.NoError(t, err)
		err = k.AddEventBtcDelegationActivated(ctx, blkHeight, fp2, del1, rAmtSat2)
		require.NoError(t, err)

		nextBlkHeight := blkHeight + 1 + datagen.RandomInt(r, 1000)
		subAmtSat2 := rAmtSat2 / 2

		err = k.AddEventBtcDelegationUnbonded(ctx, nextBlkHeight, fp2, del1, subAmtSat2)
		require.NoError(t, err)

		err = k.ProcessRewardTrackerEvents(ctx, nextBlkHeight)
		require.NoError(t, err)
		// call twice should not error out
		err = k.ProcessRewardTrackerEvents(ctx, nextBlkHeight)
		require.NoError(t, err)

		// check if the events were modified
		evts, err := k.GetOrNewRewardTrackerEvent(ctx, blkHeight)
		require.NoError(t, err)
		require.Len(t, evts.Events, 0)
		evtsNextHeight, err := k.GetOrNewRewardTrackerEvent(ctx, nextBlkHeight)
		require.NoError(t, err)
		require.Len(t, evtsNextHeight.Events, 0)

		lastProcessedHeight, err = k.GetRewardTrackerEventLastProcessedHeight(ctx)
		require.NoError(t, err)
		require.EqualValues(t, lastProcessedHeight, nextBlkHeight)

		// check if the amounts match in the reward tracker
		fp1Current, err := k.GetFinalityProviderCurrentRewards(ctx, fp1)
		require.NoError(t, err)
		require.Equal(t, fp1Current.TotalActiveSat.Uint64(), rAmtSat)

		fp2Current, err := k.GetFinalityProviderCurrentRewards(ctx, fp2)
		require.NoError(t, err)
		require.Equal(t, fp2Current.TotalActiveSat.Uint64(), rAmtSat2-subAmtSat2)

		delFp1, err := k.GetBTCDelegationRewardsTracker(ctx, fp1, del1)
		require.NoError(t, err)
		require.Equal(t, delFp1.TotalActiveSat.Uint64(), rAmtSat)
		delFp2, err := k.GetBTCDelegationRewardsTracker(ctx, fp2, del1)
		require.NoError(t, err)
		require.Equal(t, delFp2.TotalActiveSat.Uint64(), rAmtSat2-subAmtSat2)
	})
}

func FuzzSetGetOrNewRewardTrackerEvent(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2, del1 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

		blkHeight := datagen.RandomInt(r, 1000) + 2

		new, err := k.GetOrNewRewardTrackerEvent(ctx, blkHeight)
		require.NoError(t, err)
		require.Len(t, new.Events, 0)

		new.Events = append(new.Events, types.NewEventBtcDelegationActivated(fp1.String(), del1.String(), datagen.RandomMathInt(r, 1000).AddRaw(20)))
		new.Events = append(new.Events, types.NewEventBtcDelegationActivated(fp2.String(), del1.String(), datagen.RandomMathInt(r, 1000).AddRaw(20)))

		err = k.SetRewardTrackerEvent(ctx, blkHeight, new)
		require.NoError(t, err)

		old := new

		new, err = k.GetOrNewRewardTrackerEvent(ctx, blkHeight)
		require.NoError(t, err)
		require.Len(t, new.Events, 2)
		require.EqualValues(t, old, new)
	})
}

func FuzzAddRewardTrackerEventAndDeletes(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2, del1 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

		blkHeight := datagen.RandomInt(r, 1000) + 2

		err := k.AddEventBtcDelegationActivated(ctx, blkHeight, fp1, del1, datagen.RandomInt(r, 1000)+100)
		require.NoError(t, err)
		err = k.AddEventBtcDelegationActivated(ctx, blkHeight, fp2, del1, datagen.RandomInt(r, 1000)+100)
		require.NoError(t, err)
		// different height
		nextBlockHeight := blkHeight + 1 + datagen.RandomInt(r, 100)
		amtUbd := datagen.RandomInt(r, 98) + 1
		err = k.AddEventBtcDelegationUnbonded(ctx, nextBlockHeight, fp2, del1, amtUbd)
		require.NoError(t, err)

		new, err := k.GetOrNewRewardTrackerEvent(ctx, blkHeight)
		require.NoError(t, err)
		require.Len(t, new.Events, 2)

		newNext, err := k.GetOrNewRewardTrackerEvent(ctx, nextBlockHeight)
		require.NoError(t, err)
		require.Len(t, newNext.Events, 1)

		typed := newNext.Events[0].Ev.(*types.EventPowerUpdate_BtcUnbonded)
		require.Equal(t, typed.BtcUnbonded.FpAddr, fp2.String())
		require.Equal(t, typed.BtcUnbonded.TotalSat.Uint64(), amtUbd)

		// call delete twice for same height
		err = k.DeleteRewardTrackerEvents(ctx, blkHeight)
		require.NoError(t, err)
		err = k.DeleteRewardTrackerEvents(ctx, blkHeight)
		require.NoError(t, err)

		// check if there is no reward tracker there
		new, err = k.GetOrNewRewardTrackerEvent(ctx, blkHeight)
		require.NoError(t, err)
		require.Len(t, new.Events, 0)

		// check next
		err = k.DeleteRewardTrackerEvents(ctx, nextBlockHeight)
		require.NoError(t, err)

		next, err := k.GetOrNewRewardTrackerEvent(ctx, nextBlockHeight)
		require.NoError(t, err)
		require.Len(t, next.Events, 0)
	})
}
