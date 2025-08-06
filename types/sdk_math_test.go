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
			expErr:     fmt.Errorf("%w: cannot multiply coins by zero", types.ErrInvalidAmount),
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
			expErr:     fmt.Errorf("unable to create new coin -500%s: unable to validate new coin -500%s: negative coin amount: -500", appparams.DefaultBondDenom, appparams.DefaultBondDenom),
		},
		{
			title:      "overflow should error",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, datagen.NewIntMaxSupply())),
			multiplier: sdkmath.NewInt(2),
			exp:        sdk.Coins{},
			expErr: fmt.Errorf(
				"%w: unable to multiply coins %s%s by 2: %w",
				types.ErrInvalidAmount, datagen.NewIntMaxSupply().String(), appparams.DefaultBondDenom, sdkmath.ErrIntOverflow,
			),
		},
		{
			title:      "empty coins with positive multiplier",
			coins:      sdk.Coins{},
			multiplier: sdkmath.NewInt(10),
			exp:        sdk.Coins{},
			expErr:     nil,
		},
		{
			title:      "coins with zero amounts",
			coins:      sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.ZeroInt())),
			multiplier: sdkmath.NewInt(5),
			exp:        sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.ZeroInt())),
			expErr:     nil,
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

func TestSafeNewCoin(t *testing.T) {
	tcs := []struct {
		title  string
		denom  string
		amount sdkmath.Int
		expErr bool
	}{
		{
			title:  "valid coin",
			denom:  appparams.DefaultBondDenom,
			amount: sdkmath.NewInt(100),
			expErr: false,
		},
		{
			title:  "empty denom should error",
			denom:  "",
			amount: sdkmath.NewInt(100),
			expErr: true,
		},
		{
			title:  "negative amount should error",
			denom:  appparams.DefaultBondDenom,
			amount: sdkmath.NewInt(-100),
			expErr: true,
		},
		{
			title:  "zero amount is valid",
			denom:  appparams.DefaultBondDenom,
			amount: sdkmath.ZeroInt(),
			expErr: false,
		},
		{
			title:  "invalid denom characters should error",
			denom:  "INVALID-DENOM!",
			amount: sdkmath.NewInt(100),
			expErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			coin, err := types.SafeNewCoin(tc.denom, tc.amount)
			if tc.expErr {
				require.Error(t, err)
				require.Equal(t, sdk.Coin{}, coin)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.denom, coin.Denom)
			require.Equal(t, tc.amount, coin.Amount)
		})
	}
}
