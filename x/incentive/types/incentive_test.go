package types_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestGaugeValidate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	testCases := []struct {
		name      string
		gauge     *types.Gauge
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Valid gauge",
			gauge:     datagen.GenRandomGauge(r),
			expectErr: false,
		},
		{
			name: "Invalid Coins (negative amount)",
			gauge: &types.Gauge{
				Coins: sdk.Coins{{Denom: appparams.DefaultBondDenom, Amount: math.NewInt(-100)}},
			},
			expectErr: true,
			errMsg:    "gauge has invalid coins",
		},
		{
			name: "Nil Coins",
			gauge: &types.Gauge{
				Coins: sdk.Coins{sdk.Coin{}}, // Simulates a nil coin
			},
			expectErr: true,
			errMsg:    "gauge has invalid coins",
		},
		{
			name: "Empty Coins",
			gauge: &types.Gauge{
				Coins: sdk.NewCoins(),
			},
			expectErr: true,
			errMsg:    "gauge has no coins",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.gauge.Validate()
			if tc.expectErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRewardGaugeValidate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	testCases := []struct {
		name      string
		gauge     *types.RewardGauge
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Valid reward gauge",
			gauge:     datagen.GenRandomRewardGauge(r),
			expectErr: false,
		},
		{
			name: "Valid reward gauge with multiple withdrawn coins",
			gauge: &types.RewardGauge{
				Coins: sdk.NewCoins(
					sdk.NewInt64Coin(appparams.DefaultBondDenom, 100),
					sdk.NewInt64Coin("btc", 10),
				),
				WithdrawnCoins: sdk.NewCoins(
					sdk.NewInt64Coin(appparams.DefaultBondDenom, 50),
					sdk.NewInt64Coin("btc", 5),
				),
			},
			expectErr: false,
		},
		{
			name: "Invalid gauge with withdrawn coin not in Coins",
			gauge: &types.RewardGauge{
				Coins: sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 100)),
				WithdrawnCoins: sdk.NewCoins(
					sdk.NewInt64Coin(appparams.DefaultBondDenom, 50),
					sdk.NewInt64Coin("btc", 5),
				),
			},
			expectErr: true,
			errMsg:    "withdrawn coin denomination (btc) does not exist in reward coins",
		},
		{
			name: "Invalid Coins (negative amount)",
			gauge: &types.RewardGauge{
				Coins:          sdk.Coins{{Denom: appparams.DefaultBondDenom, Amount: math.NewInt(-100)}},
				WithdrawnCoins: sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 50)),
			},
			expectErr: true,
			errMsg:    "reward gauge has invalid or negative coins",
		},
		{
			name: "Invalid WithdrawnCoins (negative amount)",
			gauge: &types.RewardGauge{
				Coins:          sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 100)),
				WithdrawnCoins: sdk.Coins{{Denom: appparams.DefaultBondDenom, Amount: math.NewInt(-50)}},
			},
			expectErr: true,
			errMsg:    "reward gauge has invalid or negative withdrawn coins",
		},
		{
			name: "WithdrawnCoins exceed Coins",
			gauge: &types.RewardGauge{
				Coins:          sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 50)),
				WithdrawnCoins: sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 100)),
			},
			expectErr: true,
			errMsg:    "withdrawn coins (100ubbn) cannot exceed total coins (50ubbn)",
		},
		{
			name: "Both Coins and WithdrawnCoins are empty",
			gauge: &types.RewardGauge{
				Coins:          sdk.NewCoins(),
				WithdrawnCoins: sdk.NewCoins(),
			},
			expectErr: true,
			errMsg:    "reward gauge has no coins",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.gauge.Validate()
			if tc.expectErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStakeholderTypeValidate(t *testing.T) {
	testCases := []struct {
		name      string
		shType    types.StakeholderType
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Valid FINALITY_PROVIDER",
			shType:    types.FINALITY_PROVIDER,
			expectErr: false,
		},
		{
			name:      "Valid BTC_STAKER",
			shType:    types.BTC_STAKER,
			expectErr: false,
		},
		{
			name:      "Invalid StakeholderType",
			shType:    types.StakeholderType(999),
			expectErr: true,
			errMsg:    "invalid stakeholder type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.shType.Validate()
			if tc.expectErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}
