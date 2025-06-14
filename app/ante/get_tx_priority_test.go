package ante

import (
	"testing"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
)

// TestGetTxPriority tests the getTxPriority function
// adapted from https://github.com/celestiaorg/celestia-app/pull/2985
func TestGetTxPriority(t *testing.T) {
	denom := appparams.DefaultBondDenom

	cases := []struct {
		name        string
		fee         sdk.Coins
		gas         int64
		expectedPri int64
	}{
		{
			name:        "1 bbn fee large gas",
			fee:         sdk.NewCoins(sdk.NewInt64Coin(denom, 1_000_000)),
			gas:         1000000,
			expectedPri: 1000000,
		},
		{
			name:        "1 ubbn fee small gas",
			fee:         sdk.NewCoins(sdk.NewInt64Coin(denom, 1)),
			gas:         1,
			expectedPri: 1000000,
		},
		{
			name:        "2 ubbn fee small gas",
			fee:         sdk.NewCoins(sdk.NewInt64Coin(denom, 2)),
			gas:         1,
			expectedPri: 2000000,
		},
		{
			name:        "1_000_000 bbn fee normal gas tx",
			fee:         sdk.NewCoins(sdk.NewInt64Coin(denom, 1_000_000_000_000)),
			gas:         75000,
			expectedPri: 13333333333333,
		},
		{
			name:        "0.001 ubbn gas price",
			fee:         sdk.NewCoins(sdk.NewInt64Coin(denom, 1_000)),
			gas:         1_000_000,
			expectedPri: 1000,
		},
		{
			name:        "0 gas, should return 0 priority",
			fee:         sdk.NewCoins(sdk.NewInt64Coin(denom, 1_000)),
			gas:         0,
			expectedPri: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pri := getTxPriority(tc.fee, tc.gas)
			assert.Equal(t, tc.expectedPri, pri)
		})
	}
}
