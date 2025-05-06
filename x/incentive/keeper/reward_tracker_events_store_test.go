package keeper

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v2/x/incentive/types"
	"github.com/stretchr/testify/require"
)

func FuzzGetOrNewRewardTrackerEvent(f *testing.F) {
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
