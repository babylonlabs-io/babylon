package types_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestCoinsSafeMulInt(t *testing.T) {
	tcs := []struct {
		title      string
		coins      sdk.Coins
		multiplier sdkmath.Int
		exp        sdk.Coins
		expErr     error
	}{
		{
			title:      "multiply by zero should error",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100))),
			multiplier: sdkmath.ZeroInt(),
			exp:        nil,
			expErr:     fmt.Errorf("%s: cannot multiply by zero", types.ErrInvalidAmount),
		},
		{
			title:      "multiply single coin by positive int",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100))),
			multiplier: sdkmath.NewInt(5),
			exp:        sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(500))),
			expErr:     nil,
		},
		{
			title:      "multiply multiple coins by positive int",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100)), sdk.NewCoin("utoken", sdkmath.NewInt(200))),
			multiplier: sdkmath.NewInt(3),
			exp:        sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(300)), sdk.NewCoin("utoken", sdkmath.NewInt(600))),
			expErr:     nil,
		},
		{
			title:      "multiply by one",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100))),
			multiplier: sdkmath.NewInt(1),
			exp:        sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100))),
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
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000))),
			multiplier: sdkmath.NewInt(1000000),
			exp:        sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000000000))),
			expErr:     nil,
		},
		{
			title:      "multiply by negative int should error via SafeMul",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100))),
			multiplier: sdkmath.NewInt(-5),
			exp:        sdk.Coins{},
			expErr:     fmt.Errorf("negative coin amount: %s", "-500"),
		},
		{
			title:      "overflow should error",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, datagen.NewIntMaxSupply())),
			multiplier: sdkmath.NewInt(2),
			exp:        sdk.Coins{},
			expErr:     sdkmath.ErrIntOverflow,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			result, err := types.CoinsSafeMulInt(tc.coins, tc.multiplier)
			if tc.expErr != nil {
				require.EqualError(t, err, tc.expErr.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.exp.String(), result.String())
		})
	}
}
