package ante_test

import (
	"fmt"
	"math"
	"testing"

	bbnapp "github.com/babylonlabs-io/babylon/v2/app"
	"github.com/babylonlabs-io/babylon/v2/app/ante"
	appparams "github.com/babylonlabs-io/babylon/v2/app/params"
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

// TestCheckTxFeeWithGlobalMinGasPrices tests the CheckTxFeeWithGlobalMinGasPrices
// function
// adapted from https://github.com/celestiaorg/celestia-app/pull/2985
func TestCheckTxFeeWithGlobalMinGasPrices(t *testing.T) {
	encCfg := bbnapp.GetEncodingConfig()

	builder := encCfg.TxConfig.NewTxBuilder()
	err := builder.SetMsgs(
		banktypes.NewMsgSend(
			datagen.GenRandomAccount().GetAddress(),
			datagen.GenRandomAccount().GetAddress(),
			sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 10)),
		),
	)
	require.NoError(t, err)

	feeAmount := int64(1000)
	ctx := sdk.Context{}

	testCases := []struct {
		name       string
		fee        sdk.Coins
		gasLimit   uint64
		appVersion uint64
		expErr     bool
		errMsg     string
	}{
		{
			name:       "bad tx; fee below required minimum",
			fee:        sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, feeAmount-1)),
			gasLimit:   uint64(float64(feeAmount) / appparams.GlobalMinGasPrice),
			appVersion: uint64(2),
			expErr:     true,
		},
		{
			name:       "good tx; fee equal to required minimum",
			fee:        sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, feeAmount)),
			gasLimit:   uint64(float64(feeAmount) / appparams.GlobalMinGasPrice),
			appVersion: uint64(2),
			expErr:     false,
		},
		{
			name:       "good tx; fee above required minimum",
			fee:        sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, feeAmount+1)),
			gasLimit:   uint64(float64(feeAmount) / appparams.GlobalMinGasPrice),
			appVersion: uint64(2),
			expErr:     false,
		},
		{
			name:       "good tx; gas limit and fee are maximum values",
			fee:        sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, math.MaxInt64)),
			gasLimit:   math.MaxUint64,
			appVersion: uint64(2),
			expErr:     false,
		},
		{
			name:       "bad tx; gas limit and fee are 0",
			fee:        sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 0)),
			gasLimit:   0,
			appVersion: uint64(2),
			expErr:     true,
			errMsg:     "empty coins",
		},
		{
			name:       "good tx; minFee = 0.8, rounds up to 1",
			fee:        sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, feeAmount)),
			gasLimit:   400,
			appVersion: uint64(2),
			expErr:     false,
		},
		{
			name:       "bad tx; fee uses non-default denom",
			fee:        sdk.NewCoins(sdk.NewInt64Coin("usdt", feeAmount)),
			gasLimit:   uint64(float64(feeAmount) / appparams.GlobalMinGasPrice),
			appVersion: uint64(2),
			expErr:     true,
			errMsg:     fmt.Sprintf("only %s denom is allowed", appparams.DefaultBondDenom),
		},
		{
			name: "bad tx; fee has multiple denoms including default",
			fee: sdk.NewCoins(
				sdk.NewInt64Coin(appparams.DefaultBondDenom, feeAmount),
				sdk.NewInt64Coin("usdt", feeAmount),
			),
			gasLimit:   uint64(float64(feeAmount*2) / appparams.GlobalMinGasPrice),
			appVersion: uint64(2),
			expErr:     true,
			errMsg:     fmt.Sprintf("only %s denom is allowed", appparams.DefaultBondDenom),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder.SetGasLimit(tc.gasLimit)
			builder.SetFeeAmount(tc.fee)
			tx := builder.GetTx()

			_, _, err := ante.CheckTxFeeWithGlobalMinGasPrices(ctx, tx)
			if tc.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
