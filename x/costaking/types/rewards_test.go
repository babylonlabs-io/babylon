package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestCostakerRewardsTracker_Sanitize(t *testing.T) {
	testCases := []struct {
		name           string
		input          CostakerRewardsTracker
		expectedOutput CostakerRewardsTracker
	}{
		{
			name: "ActiveBaby is -1, should be set to 0",
			input: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.NewInt(-1),
				TotalScore:                  sdkmath.NewInt(500),
			},
			expectedOutput: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.ZeroInt(),
				TotalScore:                  sdkmath.NewInt(500),
			},
		},
		{
			name: "ActiveBaby is 0, should remain 0",
			input: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.ZeroInt(),
				TotalScore:                  sdkmath.NewInt(500),
			},
			expectedOutput: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.ZeroInt(),
				TotalScore:                  sdkmath.NewInt(500),
			},
		},
		{
			name: "ActiveBaby is positive, should remain unchanged",
			input: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.NewInt(100),
				TotalScore:                  sdkmath.NewInt(500),
			},
			expectedOutput: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.NewInt(100),
				TotalScore:                  sdkmath.NewInt(500),
			},
		},
		{
			name: "ActiveBaby is -2, should remain -2 (only -1 is sanitized)",
			input: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.NewInt(-2),
				TotalScore:                  sdkmath.NewInt(500),
			},
			expectedOutput: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              sdkmath.NewInt(1000),
				ActiveBaby:                  sdkmath.NewInt(-2),
				TotalScore:                  sdkmath.NewInt(500),
			},
		},
		{
			name: "All fields are zero, should remain unchanged",
			input: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 0,
				ActiveSatoshis:              sdkmath.ZeroInt(),
				ActiveBaby:                  sdkmath.ZeroInt(),
				TotalScore:                  sdkmath.ZeroInt(),
			},
			expectedOutput: CostakerRewardsTracker{
				StartPeriodCumulativeReward: 0,
				ActiveSatoshis:              sdkmath.ZeroInt(),
				ActiveBaby:                  sdkmath.ZeroInt(),
				TotalScore:                  sdkmath.ZeroInt(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tracker := tc.input
			tracker.Sanitize()
			require.Equal(t, tc.expectedOutput, tracker)
		})
	}
}
