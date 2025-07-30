package types

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestCoinsSafeMulInt(t *testing.T) {
	maxSupply, ok := sdkmath.NewIntFromString("115792089237316195423570985008687907853269984665640564039457584007913129639934")
	require.True(t, ok)

	tcs := []struct {
		title      string
		coins      sdk.Coins
		multiplier sdkmath.Int
		exp        sdk.Coins
		expErr     error
	}{
		{
			title:      "multiply by zero should error",
			coins:      sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100))),
			multiplier: sdkmath.ZeroInt(),
			exp:        nil,
			expErr:     fmt.Errorf("%s: cannot multiply by zero", ErrInvalidAmount),
		},
		{
			title:      "multiply single coin by positive int",
			coins:      sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100))),
			multiplier: sdkmath.NewInt(5),
			exp:        sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(500))),
			expErr:     nil,
		},
		{
			title:      "multiply multiple coins by positive int",
			coins:      sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100)), sdk.NewCoin("utoken", sdkmath.NewInt(200))),
			multiplier: sdkmath.NewInt(3),
			exp:        sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(300)), sdk.NewCoin("utoken", sdkmath.NewInt(600))),
			expErr:     nil,
		},
		{
			title:      "multiply by one",
			coins:      sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100))),
			multiplier: sdkmath.NewInt(1),
			exp:        sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100))),
			expErr:     nil,
		},
		{
			title:      "multiply empty coins",
			coins:      sdk.Coins{},
			multiplier: sdkmath.NewInt(5),
			exp:        sdk.Coins{},
			expErr:     nil,
		},
		{
			title:      "multiply large numbers",
			coins:      sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(1000000))),
			multiplier: sdkmath.NewInt(1000000),
			exp:        sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(1000000000000))),
			expErr:     nil,
		},
		{
			title:      "multiply by negative int should error via SafeMul",
			coins:      sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100))),
			multiplier: sdkmath.NewInt(-5),
			exp:        sdk.Coins{},
			expErr:     fmt.Errorf("negative coin amount: %s", "-500"),
		},
		{
			title:      "overflow should error",
			coins:      sdk.NewCoins(sdk.NewCoin("ubbn", maxSupply)),
			multiplier: sdkmath.NewInt(2),
			exp:        sdk.Coins{},
			expErr:     sdkmath.ErrIntOverflow,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			result, err := CoinsSafeMulInt(tc.coins, tc.multiplier)
			if tc.expErr != nil {
				require.EqualError(t, err, tc.expErr.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.exp.String(), result.String())
		})
	}
}
