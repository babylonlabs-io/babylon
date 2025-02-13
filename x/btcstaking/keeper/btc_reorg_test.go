package keeper_test

import (
	"testing"

	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

	k, ctx := keepertest.BTCStakingKeeper(t, nil, btcckKeeper, nil)

	err := k.SetLargestBtcReorg(ctx, p.BtcConfirmationDepth-1)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		k.HaltIfBtcReorgLargerThanConfirmationDepth(ctx)
	})

	err = k.SetLargestBtcReorg(ctx, p.BtcConfirmationDepth)
	require.NoError(t, err)
	require.Panics(t, func() {
		k.HaltIfBtcReorgLargerThanConfirmationDepth(ctx)
	})

	err = k.SetLargestBtcReorg(ctx, p.BtcConfirmationDepth+1)
	require.NoError(t, err)
	require.Panics(t, func() {
		k.HaltIfBtcReorgLargerThanConfirmationDepth(ctx)
	})
}

func TestMustGetLargestBtcReorg(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		title string

		setNewLargest   bool
		largestBtcReorg uint32
	}{
		{
			"value never set, should be zero",
			false,
			0,
		},
		{
			"value 0, should return 0",
			true,
			0,
		},
		{
			"value 15, should return 15",
			true,
			15,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			k, ctx := keepertest.BTCStakingKeeper(t, nil, nil, nil)

			if tc.setNewLargest {
				err := k.LargestBtcReorgInBlocks.Set(ctx, tc.largestBtcReorg)
				require.NoError(t, err)
			}

			actLargestBtcReorg := k.MustGetLargestBtcReorg(ctx)
			require.Equal(t, tc.largestBtcReorg, actLargestBtcReorg)
		})
	}
}

func TestSetLargestBtcReorg(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		title string

		setNewLargestFirst bool
		setLargestBtcReorg uint32

		newLargestBtcReorg      uint32
		expectedLargestBtcReorg uint32
	}{
		{
			"value never set, should be able to correctly set largest",
			false,
			0,
			2,
			2,
		},
		{
			"value before set largest: 10, set 15, should update to 15",
			true,
			10,
			15,
			15,
		},
		{
			"value before set largest: 10, set 8, should not update to 8",
			true,
			10,
			8,
			10,
		},
		{
			"value before set largest: 10, set 10, should continue to be 10",
			true,
			10,
			10,
			10,
		},
		{
			"value never set before, set 1535, should update to new value",
			false,
			0,
			1535,
			1535,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			k, ctx := keepertest.BTCStakingKeeper(t, nil, nil, nil)

			if tc.setNewLargestFirst {
				err := k.LargestBtcReorgInBlocks.Set(ctx, tc.setLargestBtcReorg)
				require.NoError(t, err)
			}

			err := k.SetLargestBtcReorg(ctx, tc.newLargestBtcReorg)
			require.NoError(t, err)

			actLargestBtcReorg := k.MustGetLargestBtcReorg(ctx)
			require.Equal(t, tc.expectedLargestBtcReorg, actLargestBtcReorg)
		})
	}
}
