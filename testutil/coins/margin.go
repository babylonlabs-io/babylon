package coins

import (
	"strings"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func RequireIntDiffInPointOnePercentMargin(t *testing.T, v1, v2 math.Int, logs ...string) {
	inMargin := IntDiffInPointOnePercentMargin(v1, v2)
	if !inMargin {
		t.Logf("Int are not in 0.1 percent margin: %s", strings.Join(logs, ","))
		t.Logf("v1: %s", v1.String())
		t.Logf("v2: %s", v2.String())
	}
	require.True(t, inMargin)
}

func RequireCoinsDiffInPointOnePercentMargin(t *testing.T, c1, c2 sdk.Coins) {
	RequireCoinsDiffInMargin(t, c1, c2, 1)
}

// RequireCoinsDiffInMargin checks that the difference between two coin sets
// is within `permille` parts per thousand of c1. For example, permille=1
// means 0.1% margin, permille=10 means 1% margin.
func RequireCoinsDiffInMargin(t *testing.T, c1, c2 sdk.Coins, permille int64) {
	inMargin := CoinsDiffInMargin(c1, c2, permille)
	if !inMargin {
		marginPct := float64(permille) / 10.0
		t.Logf("Coins are not in %.1f%% margin", marginPct)
		t.Logf("c1: %s", c1.String())
		t.Logf("c2: %s", c2.String())
	}
	require.True(t, inMargin)
}

func IntDiffInPointOnePercentMargin(c1, c2 math.Int) bool {
	diff := c1.Sub(c2).Abs()
	margin := CalculatePointOnePercentOrMinOneForInt(c1)
	return margin.GTE(diff)
}

func CoinsDiffInPointOnePercentMargin(c1, c2 sdk.Coins) bool {
	return CoinsDiffInMargin(c1, c2, 1)
}

// CoinsDiffInMargin checks if c1 and c2 are within `permille` parts per
// thousand of each other. permille=1 is 0.1%, permille=10 is 1%.
func CoinsDiffInMargin(c1, c2 sdk.Coins, permille int64) bool {
	diff, hasNeg := c1.SafeSub(c2...)
	if hasNeg {
		diff = AbsoluteCoins(diff)
	}
	margin := CalculateMarginOrMinOne(c1, permille)
	return margin.IsAllGTE(diff)
}

func AbsoluteCoins(value sdk.Coins) sdk.Coins {
	ret := sdk.NewCoins()
	for _, v := range value {
		ret = ret.Add(sdk.NewCoin(v.Denom, v.Amount.Abs()))
	}
	return ret
}

// CalculatePointOnePercentOrMinOne transforms 10000 = 10
// if 0.1% is of the value is less 1, sets one as value in the coin
func CalculatePointOnePercentOrMinOne(value sdk.Coins) sdk.Coins {
	return CalculateMarginOrMinOne(value, 1)
}

// CalculateMarginOrMinOne calculates `permille` parts per thousand of value.
// permille=1 is 0.1% (1/1000), permille=10 is 1% (10/1000).
// If the result is less than 1, sets one as the minimum value.
func CalculateMarginOrMinOne(value sdk.Coins, permille int64) sdk.Coins {
	numerator := math.NewInt(permille)
	denominator := math.NewInt(1000)
	result := value.MulInt(numerator).QuoInt(denominator)
	return coinsAtLeastMinAmount(result, math.OneInt())
}

// CalculatePointOnePercentOrMinOneForInt transforms 10000 = 10
// if 0.1% is of the value is less 1, sets one as value in the coin
func CalculatePointOnePercentOrMinOneForInt(value math.Int) math.Int {
	numerator := math.NewInt(1)
	denominator := math.NewInt(1000)
	result := value.Mul(numerator).Quo(denominator)
	return math.MaxInt(result, math.OneInt())
}

// CalculatePercentageOfCoins if percentage is 30, transforms 100bbn = 30bbn
func CalculatePercentageOfCoins(value sdk.Coins, percentage uint64) sdk.Coins {
	divisor := math.NewInt(100)
	multiplier := math.NewIntFromUint64(percentage)
	result := value.MulInt(multiplier).QuoInt(divisor)
	return result
}

func coinsAtLeastMinAmount(value sdk.Coins, minAmt math.Int) sdk.Coins {
	ret := sdk.NewCoins()
	for _, v := range value {
		minCoinAmt := coinAtLeastMinAmount(v, minAmt)
		ret = ret.Add(minCoinAmt)
	}
	return ret
}

func coinAtLeastMinAmount(v sdk.Coin, minAmt math.Int) sdk.Coin {
	if v.Amount.GT(minAmt) {
		return v
	}
	return sdk.NewCoin(v.Denom, minAmt)
}
