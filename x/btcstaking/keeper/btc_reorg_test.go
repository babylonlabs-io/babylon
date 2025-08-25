package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestHaltIfBtcReorgLargerThanConfirmationDepth(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := btccheckpointtypes.DefaultParams()
	btcckKeeper := types.NewMockBtcCheckpointKeeper(ctrl)

	btcckKeeper.EXPECT().GetParams(gomock.Any()).Return(p).AnyTimes()

	k, ctx := keepertest.BTCStakingKeeper(t, nil, btcckKeeper, nil, nil)
	r := rand.New(rand.NewSource(time.Now().Unix()))

	from, to := datagen.GenRandomBTCHeaderInfo(r), datagen.GenRandomBTCHeaderInfo(r)
	largestReorg := types.NewLargestBtcReOrg(from, to)

	largestReorg.BlockDiff = p.BtcConfirmationDepth - 1
	err := k.SetLargestBtcReorg(ctx, largestReorg)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		k.HaltIfBtcReorgLargerThanConfirmationDepth(ctx)
	})

	largestReorg.BlockDiff = p.BtcConfirmationDepth
	err = k.SetLargestBtcReorg(ctx, largestReorg)
	require.NoError(t, err)
	require.Panics(t, func() {
		k.HaltIfBtcReorgLargerThanConfirmationDepth(ctx)
	})

	largestReorg.BlockDiff = p.BtcConfirmationDepth + 1
	err = k.SetLargestBtcReorg(ctx, largestReorg)
	require.NoError(t, err)
	require.Panics(t, func() {
		k.HaltIfBtcReorgLargerThanConfirmationDepth(ctx)
	})
}

func TestMustGetLargestBtcReorg(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().Unix()))

	from, to := datagen.GenRandomBTCHeaderInfo(r), datagen.GenRandomBTCHeaderInfo(r)

	tcs := []struct {
		title string

		setNewLargest   bool
		largestBtcReorg *types.LargestBtcReOrg
	}{
		{
			"value never set, should be zero",
			false,
			nil,
		},
		{
			"value 0, should return 0",
			true,
			&types.LargestBtcReOrg{
				BlockDiff:    0,
				RollbackFrom: from,
				RollbackTo:   to,
			},
		},
		{
			"value 15, should return 15",
			true,
			&types.LargestBtcReOrg{
				BlockDiff:    15,
				RollbackFrom: from,
				RollbackTo:   to,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			k, ctx := keepertest.BTCStakingKeeper(t, nil, nil, nil, nil)

			if tc.setNewLargest {
				err := k.LargestBtcReorg.Set(ctx, *tc.largestBtcReorg)
				require.NoError(t, err)
			}

			actLargestBtcReorg := k.GetLargestBtcReorg(ctx)
			require.Equal(t, tc.largestBtcReorg, actLargestBtcReorg)
		})
	}
}

func TestSetLargestBtcReorg(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(time.Now().Unix()))
	from, to := datagen.GenRandomBTCHeaderInfo(r), datagen.GenRandomBTCHeaderInfo(r)

	tcs := []struct {
		title string

		setNewLargestFirst     bool
		setLargestBtcReorgDiff uint32

		newLargestBtcReorgDiff  uint32
		expectedLargestBtcReorg *types.LargestBtcReOrg
	}{
		{
			"value never set, should be able to correctly set largest",
			false,
			0,
			2,
			&types.LargestBtcReOrg{
				BlockDiff:    2,
				RollbackFrom: from,
				RollbackTo:   to,
			},
		},
		{
			"value before set largest: 10, set 15, should update to 15",
			true,
			10,
			15,
			&types.LargestBtcReOrg{
				BlockDiff:    15,
				RollbackFrom: from,
				RollbackTo:   to,
			},
		},
		{
			"value before set largest: 10, set 8, should not update to 8",
			true,
			10,
			8,
			&types.LargestBtcReOrg{
				BlockDiff:    10,
				RollbackFrom: from,
				RollbackTo:   to,
			},
		},
		{
			"value before set largest: 10, set 10, should continue to be 10",
			true,
			10,
			10,
			&types.LargestBtcReOrg{
				BlockDiff:    10,
				RollbackFrom: from,
				RollbackTo:   to,
			},
		},
		{
			"value never set before, set 1535, should update to new value",
			false,
			0,
			1535,
			&types.LargestBtcReOrg{
				BlockDiff:    1535,
				RollbackFrom: from,
				RollbackTo:   to,
			},
		},
	}

	largestReorg := types.NewLargestBtcReOrg(from, to)

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			k, ctx := keepertest.BTCStakingKeeper(t, nil, nil, nil, nil)

			if tc.setNewLargestFirst {
				largestReorg.BlockDiff = tc.setLargestBtcReorgDiff
				err := k.LargestBtcReorg.Set(ctx, largestReorg)
				require.NoError(t, err)
			}

			largestReorg.BlockDiff = tc.newLargestBtcReorgDiff
			err := k.SetLargestBtcReorg(ctx, largestReorg)
			require.NoError(t, err)

			actLargestBtcReorg := k.GetLargestBtcReorg(ctx)
			require.EqualValues(t, tc.expectedLargestBtcReorg, actLargestBtcReorg)
		})
	}
}
