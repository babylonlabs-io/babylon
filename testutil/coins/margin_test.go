package coins_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/coins"
)

const (
	denom = "xsss"
)

func TestCalculatePointOnePercent(t *testing.T) {
	result := coins.CalculatePointOnePercentOrMinOne(sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(10000))))
	require.Equal(t, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(10))).String(), result.String())
}

func TestCalculatePercentageOfCoins(t *testing.T) {
	result := coins.CalculatePercentageOfCoins(sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(100))), 30)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(30))).String(), result.String())

	result = coins.CalculatePercentageOfCoins(sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(1560))), 30)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(468))).String(), result.String())
}

func TestCoinsDiffInPointOnePercentMargin(t *testing.T) {
	c1 := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(10000)))
	c2 := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(9999)))
	require.True(t, coins.CoinsDiffInPointOnePercentMargin(c1, c2))

	limitC2 := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(9990)))
	require.True(t, coins.CoinsDiffInPointOnePercentMargin(c1, limitC2))

	badC2 := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(9989)))
	require.False(t, coins.CoinsDiffInPointOnePercentMargin(c1, badC2))
}
