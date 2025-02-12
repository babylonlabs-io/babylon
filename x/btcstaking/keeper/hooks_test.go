package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/collections"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	ltypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/stretchr/testify/require"
)

func TestAfterBTCRollBack(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().Unix()))

	tcs := []struct {
		title                 string
		rollbackFrom          *ltypes.BTCHeaderInfo
		rollbackTo            *ltypes.BTCHeaderInfo
		expLargestBtcReorg    uint32
		expLargestBtcReorgErr error
	}{
		{
			"No rollback 'from'",
			nil,
			datagen.GenRandomBTCHeaderInfo(r),
			0,
			fmt.Errorf("%w: key 'no_key' of type uint32", collections.ErrNotFound),
		},
		{
			"No rollback 'to'",
			datagen.GenRandomBTCHeaderInfo(r),
			nil,
			0,
			fmt.Errorf("%w: key 'no_key' of type uint32", collections.ErrNotFound),
		},
		{
			"Rollback 'from' height > 'to' height",
			&ltypes.BTCHeaderInfo{
				Height: 10,
			},
			&ltypes.BTCHeaderInfo{
				Height: 12,
			},
			0,
			fmt.Errorf("%w: key 'no_key' of type uint32", collections.ErrNotFound),
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
			nil,
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
			nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			k, ctx := keepertest.BTCStakingKeeper(t, nil, nil, nil)
			k.Hooks().AfterBTCRollBack(ctx, tc.rollbackFrom, tc.rollbackTo)

			actLargestBtcReorg, err := k.LargestBtcReorgInBlocks.Get(ctx)
			if tc.expLargestBtcReorgErr != nil {
				require.EqualError(t, err, tc.expLargestBtcReorgErr.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expLargestBtcReorg, actLargestBtcReorg)
		})
	}
}
