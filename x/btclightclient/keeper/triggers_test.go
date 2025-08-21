package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

<<<<<<< HEAD
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btclightclient/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/test-go/testify/require"
=======
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/btclightclient/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/stretchr/testify/require"
>>>>>>> dfbd055 (chore:  e2e refactory (#1552))
)

func TestCheckRollBackInvariants(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().Unix()))

	randHeaderFrom := datagen.GenRandomBTCHeaderInfo(r)
	randHeaderTo := datagen.GenRandomBTCHeaderInfo(r)
	tcs := []struct {
		title        string
		rollbackFrom *types.BTCHeaderInfo
		rollbackTo   *types.BTCHeaderInfo
		expErr       error
	}{
		{
			"No rollback 'from'",
			nil,
			datagen.GenRandomBTCHeaderInfo(r),
			fmt.Errorf("Call BTC rollback without tip"),
		},
		{
			"No rollback 'to'",
			datagen.GenRandomBTCHeaderInfo(r),
			nil,
			fmt.Errorf("Call BTC rollback without rollbackTo"),
		},
		{
			"Rollback 'from' height > 'to' height",
			&types.BTCHeaderInfo{
				Height: 10,
				Hash:   randHeaderFrom.Hash,
			},
			&types.BTCHeaderInfo{
				Height: 12,
				Hash:   randHeaderTo.Hash,
			},
			fmt.Errorf(
				"BTC rollback with rollback 'To' higher or equal than 'From'\n%s\n%s",
				fmt.Sprintf("'From' -> %d - %s", 10, randHeaderFrom.Hash.MarshalHex()),
				fmt.Sprintf("'To' -> %d - %s", 12, randHeaderTo.Hash.MarshalHex()),
			),
		},
		{
			"Rollback 'from' height == 'to' height",
			&types.BTCHeaderInfo{
				Height: 18,
				Hash:   randHeaderFrom.Hash,
			},
			&types.BTCHeaderInfo{
				Height: 18,
				Hash:   randHeaderTo.Hash,
			},
			fmt.Errorf(
				"BTC rollback with rollback 'To' higher or equal than 'From'\n%s\n%s",
				fmt.Sprintf("'From' -> %d - %s", 18, randHeaderFrom.Hash.MarshalHex()),
				fmt.Sprintf("'To' -> %d - %s", 18, randHeaderTo.Hash.MarshalHex()),
			),
		},
		{
			"Rollback to correct height",
			&types.BTCHeaderInfo{
				Height: 15,
			},
			&types.BTCHeaderInfo{
				Height: 12,
			},
			nil,
		},
		{
			"Rollback to very large height",
			&types.BTCHeaderInfo{
				Height: 15000,
			},
			&types.BTCHeaderInfo{
				Height: 12,
			},
			nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			actErr := keeper.CheckRollBackInvariants(tc.rollbackFrom, tc.rollbackTo)
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}

			require.NoError(t, actErr)
		})
	}
}
