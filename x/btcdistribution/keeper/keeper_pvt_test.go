package keeper

import (
	"testing"

	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestWeightStaked(t *testing.T) {
	// C_btc = S_btc / S_btc_total
	// C_bbn = S_bbn / S_bbn_total
	// g(C_bbn / C_btc)

	// C_btc = 5 / 10 = 0.5
	// C_bbn = 50 / 100 = 0.5
	// g(C_bbn / C_btc) = 1 with decimals => 100000000
	v := weightStaked(
		math.NewInt(100), math.NewInt(10),
		addDecimals(math.NewInt(50)), addDecimals(math.NewInt(5)),
	)
	require.Equal(t, "100000000", v.String())

	// C_btc = 5 / 10 = 0.5
	// C_bbn = 10 / 100 = 0.1
	// g(C_bbn / C_btc) = 0,2 with decimals => 20000000
	v = weightStaked(
		math.NewInt(100), math.NewInt(10),
		addDecimals(math.NewInt(10)), addDecimals(math.NewInt(5)),
	)
	require.Equal(t, "20000000", v.String())
}

func TestRewardRatio(t *testing.T) {
	coins := sdk.NewCoins(
		sdk.NewCoin(appparams.BaseCoinUnit, math.NewInt(10_000000)),
	)

	ratioRwd := rewardRatio(coins, math.NewInt(100), addDecimals(math.NewInt(20)))
	require.Equal(t, "2000000ubbn", ratioRwd.String())

	ratioRwd = rewardRatio(coins, math.NewInt(1000), addDecimals(math.NewInt(350)))
	require.Equal(t, "3500000ubbn", ratioRwd.String())

	ratioRwd = rewardRatio(coins, math.NewInt(1000), addDecimals(math.NewInt(35)))
	require.Equal(t, "350000ubbn", ratioRwd.String())
}
