package keeper_test

import (
	"testing"

	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	ltypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/stretchr/testify/require"
)

func TestAfterBTCRollBack(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		title              string
		rollbackFrom       *ltypes.BTCHeaderInfo
		rollbackTo         *ltypes.BTCHeaderInfo
		expLargestBtcReorg uint32
	}{
		{
			"Rollback 'from' height > 'to' height",
			&ltypes.BTCHeaderInfo{
				Height: 10,
			},
			&ltypes.BTCHeaderInfo{
				Height: 12,
			},
			// uint32 when subtracts
			4294967294,
		},
		{
			"Rollback to correct height",
			&ltypes.BTCHeaderInfo{
				Height: 15,
			},
			&ltypes.BTCHeaderInfo{
				Height: 12,
			},
			3,
		},
		{
			"Rollback to very large height",
			&ltypes.BTCHeaderInfo{
				Height: 15000,
			},
			&ltypes.BTCHeaderInfo{
				Height: 12,
			},
			14988,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			k, ctx := keepertest.BTCStakingKeeper(t, nil, nil, nil, nil)
			k.Hooks().AfterBTCRollBack(ctx, tc.rollbackFrom, tc.rollbackTo)

			actLargestBtcReorg, err := k.LargestBtcReorg.Get(ctx)
			require.NoError(t, err)
			require.EqualValues(t, tc.expLargestBtcReorg, actLargestBtcReorg.BlockDiff)
		})
	}
}
