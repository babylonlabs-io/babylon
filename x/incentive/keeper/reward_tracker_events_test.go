package keeper

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

func TestGetRewardTrackerEventsCompiledByBtcDel(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	del1, del2, del3 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

	// since we can set the last processed height, each test case set a higher last processed block height
	tcs := []struct {
		name            string
		setup           func() (untilBlkHeight uint64)
		filter          func(fpAddr string) bool
		expectedResults map[string]int64
	}{
		{
			name: "empty events - no processed height set",
			setup: func() uint64 {
				return 100
			},
			filter:          nil,
			expectedResults: map[string]int64{},
		},
		{
			name: "empty events - processed height equals query height",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 50)
				require.NoError(t, err)
				return 50
			},
			filter:          nil,
			expectedResults: map[string]int64{},
		},
		{
			name: "single activation event",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 10)
				require.NoError(t, err)

				err = k.AddEventBtcDelegationActivated(ctx, 15, fp1, del1, 1000)
				require.NoError(t, err)

				return 20
			},
			filter: nil,
			expectedResults: map[string]int64{
				del1.String(): 1000,
			},
		},
		{
			name: "single unbonding event",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 20)
				require.NoError(t, err)

				err = k.AddEventBtcDelegationUnbonded(ctx, 25, fp1, del1, 300)
				require.NoError(t, err)

				return 30
			},
			filter: nil,
			expectedResults: map[string]int64{
				del1.String(): -300,
			},
		},
		{
			name: "multiple events for same delegation - net positive",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 30)
				require.NoError(t, err)

				err = k.AddEventBtcDelegationActivated(ctx, 35, fp1, del1, 1500) // + 1500
				require.NoError(t, err)

				err = k.AddEventBtcDelegationUnbonded(ctx, 36, fp2, del1, 600) // - 600
				require.NoError(t, err)

				err = k.AddEventBtcDelegationActivated(ctx, 37, fp1, del1, 200) // + 200
				require.NoError(t, err)

				return 40
			},
			filter: nil,
			expectedResults: map[string]int64{
				del1.String(): 1100, // 1500 - 600 + 200 = 1100
			},
		},
		{
			name: "multiple events for same delegation - net negative",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 40)
				require.NoError(t, err)

				err = k.AddEventBtcDelegationActivated(ctx, 45, fp1, del2, 500) // + 500
				require.NoError(t, err)

				err = k.AddEventBtcDelegationUnbonded(ctx, 46, fp1, del2, 800) // - 800
				require.NoError(t, err)

				return 50
			},
			filter: nil,
			expectedResults: map[string]int64{
				del2.String(): -300, // 500 - 800 = -300
			},
		},
		{
			name: "multiple delegations with different finality providers",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 50)
				require.NoError(t, err)

				// del1 with fp1: +2000
				err = k.AddEventBtcDelegationActivated(ctx, 55, fp1, del1, 2000)
				require.NoError(t, err)

				// del2 with fp2: +1500, -500 = +1000
				err = k.AddEventBtcDelegationActivated(ctx, 56, fp2, del2, 1500)
				require.NoError(t, err)
				err = k.AddEventBtcDelegationUnbonded(ctx, 57, fp2, del2, 500)
				require.NoError(t, err)

				// del3 with fp1: +800, -1200 = -400
				err = k.AddEventBtcDelegationActivated(ctx, 58, fp1, del3, 800)
				require.NoError(t, err)
				err = k.AddEventBtcDelegationUnbonded(ctx, 59, fp1, del3, 1200)
				require.NoError(t, err)

				return 60
			},
			filter: nil,
			expectedResults: map[string]int64{
				del1.String(): 2000,
				del2.String(): 1000,
				del3.String(): -400,
			},
		},
		{
			name: "events across multiple block heights",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 60)
				require.NoError(t, err)

				// Height 65: del1 +1000
				err = k.AddEventBtcDelegationActivated(ctx, 65, fp1, del1, 1000)
				require.NoError(t, err)

				// Height 70: del1 -300, del2 +600
				err = k.AddEventBtcDelegationUnbonded(ctx, 70, fp1, del1, 300)
				require.NoError(t, err)
				err = k.AddEventBtcDelegationActivated(ctx, 70, fp2, del2, 600)
				require.NoError(t, err)

				// Height 90 will not take effect as it should only go until 80
				err = k.AddEventBtcDelegationUnbonded(ctx, 81, fp2, del2, 100)
				require.NoError(t, err)

				return 80
			},
			filter: nil,
			expectedResults: map[string]int64{
				del1.String(): 700, // 1000 - 300 = 700
				del2.String(): 600, // 600 = 600
			},
		},
		{
			name: "events exactly cancel out",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 81)
				require.NoError(t, err)

				amt := uint64(100)
				err = k.AddEventBtcDelegationActivated(ctx, 85, fp1, del1, amt)
				require.NoError(t, err)
				err = k.AddEventBtcDelegationUnbonded(ctx, 86, fp1, del1, amt)
				require.NoError(t, err)

				return 90
			},
			filter: nil,
			expectedResults: map[string]int64{
				del1.String(): 0,
			},
		},
		{
			name: "filter fp1 only - excludes fp2 events",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 90)
				require.NoError(t, err)

				// fp1 events - should be included
				err = k.AddEventBtcDelegationActivated(ctx, 95, fp1, del1, 1000)
				require.NoError(t, err)
				err = k.AddEventBtcDelegationActivated(ctx, 96, fp1, del2, 500)
				require.NoError(t, err)

				// fp2 events - should be excluded
				err = k.AddEventBtcDelegationActivated(ctx, 97, fp2, del1, 2000)
				require.NoError(t, err)
				err = k.AddEventBtcDelegationActivated(ctx, 98, fp2, del3, 800)
				require.NoError(t, err)

				return 100
			},
			filter: func(fpAddr string) bool { return fpAddr == fp1.String() }, // Only fp1
			expectedResults: map[string]int64{
				del1.String(): 1000, // Only fp1 event
				del2.String(): 500,
				// del3 not included because fp2 events are filtered out
			},
		},
		{
			name: "filter fp2 only - excludes fp1 events",
			setup: func() uint64 {
				err := k.SetRewardTrackerEventLastProcessedHeight(ctx, 100)
				require.NoError(t, err)

				// fp1 events - should be excluded
				err = k.AddEventBtcDelegationActivated(ctx, 105, fp1, del1, 1500)
				require.NoError(t, err)

				// fp2 events - should be included
				err = k.AddEventBtcDelegationActivated(ctx, 106, fp2, del1, 800)
				require.NoError(t, err)
				err = k.AddEventBtcDelegationUnbonded(ctx, 107, fp2, del2, 200)
				require.NoError(t, err)

				return 110
			},
			filter: func(fpAddr string) bool { return fpAddr == fp2.String() }, // Only fp2
			expectedResults: map[string]int64{
				del1.String(): 800, // Only fp2 event
				del2.String(): -200,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			untilHeight := tc.setup()

			result, err := k.GetRewardTrackerEventsCompiledByBtcDel(ctx, untilHeight, tc.filter)
			require.NoError(t, err)

			require.Len(t, result, len(tc.expectedResults))
			for delAddr, expectedSats := range tc.expectedResults {
				actualSats, exists := result[delAddr]
				require.True(t, exists, "delegation address %s should exist in results", delAddr)
				require.Equal(t, expectedSats, actualSats.Int64(), "incorrect sats for delegation %s", delAddr)
			}

			// check that there are no extra entries
			for delAddr := range result {
				_, expected := tc.expectedResults[delAddr]
				require.True(t, expected, "unexpected delegation address %s in results", delAddr)
			}
		})
	}
}

func FuzzGetRewardTrackerEventsCompiledByBtcDel(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
		del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		lastProcessedHeight := datagen.RandomInt(r, 100) + 1
		err := k.SetRewardTrackerEventLastProcessedHeight(ctx, lastProcessedHeight)
		require.NoError(t, err)

		numEvents := datagen.RandomInt(r, 10) + 1
		expectedSatsAll := make(map[string]int64)
		expectedSatsFp1Only := make(map[string]int64)

		for i := uint64(0); i < numEvents; i++ {
			height := lastProcessedHeight + i + 1

			delIdx, fpIdx := r.Intn(2), r.Intn(2)
			selectedDel := [2]sdk.AccAddress{del1, del2}[delIdx]
			selectedFp := [2]sdk.AccAddress{fp1, fp2}[fpIdx]

			amt := datagen.RandomInt(r, 1000) + 1

			if r.Intn(10) > 4 { // 60% of chance to activate
				err = k.AddEventBtcDelegationActivated(ctx, height, selectedFp, selectedDel, amt)
				require.NoError(t, err)
				expectedSatsAll[selectedDel.String()] += int64(amt)
				if selectedFp.Equals(fp1) {
					expectedSatsFp1Only[selectedDel.String()] += int64(amt)
				}
				continue
			}

			err = k.AddEventBtcDelegationUnbonded(ctx, height, selectedFp, selectedDel, amt)
			require.NoError(t, err)
			expectedSatsAll[selectedDel.String()] -= int64(amt)
			if selectedFp.Equals(fp1) {
				expectedSatsFp1Only[selectedDel.String()] -= int64(amt)
			}
		}

		// Test with no filter (include all FPs)
		result, err := k.GetRewardTrackerEventsCompiledByBtcDel(ctx, lastProcessedHeight+numEvents, nil)
		require.NoError(t, err)

		for delAddr, expectedAmount := range expectedSatsAll {
			actualSats, exists := result[delAddr]
			require.True(t, exists, "delegation %s should exist in results", delAddr)
			require.Equal(t, expectedAmount, actualSats.Int64())
		}

		// Test with fp1 filter only
		fp1Filter := func(fpAddr string) bool { return fpAddr == fp1.String() }
		resultFp1, err := k.GetRewardTrackerEventsCompiledByBtcDel(ctx, lastProcessedHeight+numEvents, fp1Filter)
		require.NoError(t, err)

		for delAddr, expectedAmount := range expectedSatsFp1Only {
			actualSats, exists := resultFp1[delAddr]
			require.True(t, exists, "delegation %s should exist in fp1-filtered results", delAddr)
			require.Equal(t, expectedAmount, actualSats.Int64())
		}

		// Ensure fp2 events are excluded from fp1-filtered results
		for delAddr := range resultFp1 {
			_, expectedInFp1 := expectedSatsFp1Only[delAddr]
			require.True(t, expectedInFp1, "unexpected delegation %s in fp1-filtered results", delAddr)
		}
	})
}
