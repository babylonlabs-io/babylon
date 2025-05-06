package keeper

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v2/x/incentive/types"
	"github.com/stretchr/testify/require"
)

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
		err = k.AddEventBtcDelegationUnbonded(ctx, nextBlockHeight, fp2, del1, datagen.RandomInt(r, 98)+1)
		require.NoError(t, err)

		new, err := k.GetOrNewRewardTrackerEvent(ctx, blkHeight)
		require.NoError(t, err)
		require.Len(t, new.Events, 2)

		newNext, err := k.GetOrNewRewardTrackerEvent(ctx, nextBlockHeight)
		require.NoError(t, err)
		require.Len(t, newNext.Events, 1)

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
