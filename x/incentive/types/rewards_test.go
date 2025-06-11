package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestFinalityProviderCurrentRewards_Validate(t *testing.T) {
	validCoins := sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 100))
	validTotalActiveSat := math.NewInt(1000)

	tests := []struct {
		name           string
		rewards        types.FinalityProviderCurrentRewards
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid finality provider current rewards",
			rewards: types.FinalityProviderCurrentRewards{
				CurrentRewards: validCoins,
				Period:         5,
				TotalActiveSat: validTotalActiveSat,
			},
			expectError: false,
		},
		{
			name: "invalid coins - negative amount",
			rewards: types.FinalityProviderCurrentRewards{
				CurrentRewards: sdk.Coins{{Denom: appparams.DefaultBondDenom, Amount: math.NewInt(-100)}},
				Period:         5,
				TotalActiveSat: validTotalActiveSat,
			},
			expectError:    true,
			expectedErrMsg: "current rewards has invalid coins",
		},
		{
			name: "nil coins",
			rewards: types.FinalityProviderCurrentRewards{
				CurrentRewards: sdk.Coins{sdk.Coin{}}, // This creates a nil coin
				Period:         5,
				TotalActiveSat: validTotalActiveSat,
			},
			expectError:    true,
			expectedErrMsg: "current rewards has invalid coins",
		},
		{
			name: "empty coins",
			rewards: types.FinalityProviderCurrentRewards{
				CurrentRewards: sdk.NewCoins(),
				Period:         5,
				TotalActiveSat: validTotalActiveSat,
			},
			expectError:    true,
			expectedErrMsg: "current rewards has no coins",
		},
		{
			name: "nil total active sat",
			rewards: types.FinalityProviderCurrentRewards{
				CurrentRewards: validCoins,
				Period:         5,
				TotalActiveSat: math.Int{}, // This creates a nil Int
			},
			expectError:    true,
			expectedErrMsg: "current rewards has no total active satoshi delegated",
		},
		{
			name: "negative total active sat",
			rewards: types.FinalityProviderCurrentRewards{
				CurrentRewards: validCoins,
				Period:         5,
				TotalActiveSat: math.NewInt(-1000),
			},
			expectError:    true,
			expectedErrMsg: "current rewards has a negative total active satoshi delegated value",
		},
		{
			name: "zero values are invalid",
			rewards: types.FinalityProviderCurrentRewards{
				CurrentRewards: sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 1)), // At least one coin
				Period:         0,
				TotalActiveSat: math.NewInt(0),
			},
			expectError:    true,
			expectedErrMsg: "fp current rewards period must be positive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rewards.Validate()

			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
