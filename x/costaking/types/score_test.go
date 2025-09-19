package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestCostakerRewardsTracker_UpdateScore(t *testing.T) {
	tcs := []struct {
		name                string
		activeSatoshis      sdkmath.Int
		activeBaby          sdkmath.Int
		initialTotalScore   sdkmath.Int
		scoreRatioBtcByBaby sdkmath.Int
		expectedTotalScore  sdkmath.Int
		expectedDelta       sdkmath.Int
	}{
		{
			name:                "BTC is limiting factor",
			activeSatoshis:      sdkmath.NewInt(1000),
			activeBaby:          sdkmath.NewInt(5000),
			initialTotalScore:   sdkmath.NewInt(500),
			scoreRatioBtcByBaby: sdkmath.NewInt(2), // baby/ratio = 5000/2 = 2500, min(1000, 2500) = 1000
			expectedTotalScore:  sdkmath.NewInt(1000),
			expectedDelta:       sdkmath.NewInt(500), // 1000 - 500 = 500
		},
		{
			name:                "Baby is limiting factor",
			activeSatoshis:      sdkmath.NewInt(3000),
			activeBaby:          sdkmath.NewInt(2000),
			initialTotalScore:   sdkmath.NewInt(800),
			scoreRatioBtcByBaby: sdkmath.NewInt(2), // baby/ratio = 2000/2 = 1000, min(3000, 1000) = 1000
			expectedTotalScore:  sdkmath.NewInt(1000),
			expectedDelta:       sdkmath.NewInt(200), // 1000 - 800 = 200
		},
		{
			name:                "Score decreases (negative delta)",
			activeSatoshis:      sdkmath.NewInt(500),
			activeBaby:          sdkmath.NewInt(1000),
			initialTotalScore:   sdkmath.NewInt(800),
			scoreRatioBtcByBaby: sdkmath.NewInt(2), // baby/ratio = 1000/2 = 500, min(500, 500) = 500
			expectedTotalScore:  sdkmath.NewInt(500),
			expectedDelta:       sdkmath.NewInt(-300), // 500 - 800 = -300
		},
		{
			name:                "Zero active satoshis",
			activeSatoshis:      sdkmath.ZeroInt(),
			activeBaby:          sdkmath.NewInt(1000),
			initialTotalScore:   sdkmath.NewInt(100),
			scoreRatioBtcByBaby: sdkmath.NewInt(2), // baby/ratio = 1000/2 = 500, min(0, 500) = 0
			expectedTotalScore:  sdkmath.ZeroInt(),
			expectedDelta:       sdkmath.NewInt(-100), // 0 - 100 = -100
		},
		{
			name:                "Zero active baby",
			activeSatoshis:      sdkmath.NewInt(1000),
			activeBaby:          sdkmath.ZeroInt(),
			initialTotalScore:   sdkmath.NewInt(200),
			scoreRatioBtcByBaby: sdkmath.NewInt(2), // baby/ratio = 0/2 = 0, min(1000, 0) = 0
			expectedTotalScore:  sdkmath.ZeroInt(),
			expectedDelta:       sdkmath.NewInt(-200), // 0 - 200 = -200
		},
		{
			name:                "Equal amounts with ratio 1",
			activeSatoshis:      sdkmath.NewInt(1000),
			activeBaby:          sdkmath.NewInt(1000),
			initialTotalScore:   sdkmath.ZeroInt(),
			scoreRatioBtcByBaby: sdkmath.NewInt(1), // baby/ratio = 1000/1 = 1000, min(1000, 1000) = 1000
			expectedTotalScore:  sdkmath.NewInt(1000),
			expectedDelta:       sdkmath.NewInt(1000), // 1000 - 0 = 1000
		},
		{
			name:                "High ratio favors baby",
			activeSatoshis:      sdkmath.NewInt(1000),
			activeBaby:          sdkmath.NewInt(10000),
			initialTotalScore:   sdkmath.NewInt(500),
			scoreRatioBtcByBaby: sdkmath.NewInt(100), // baby/ratio = 10000/100 = 100, min(1000, 100) = 100
			expectedTotalScore:  sdkmath.NewInt(100),
			expectedDelta:       sdkmath.NewInt(-400), // 100 - 500 = -400
		},
		{
			name:                "No change in score",
			activeSatoshis:      sdkmath.NewInt(500),
			activeBaby:          sdkmath.NewInt(1000),
			initialTotalScore:   sdkmath.NewInt(500),
			scoreRatioBtcByBaby: sdkmath.NewInt(2), // baby/ratio = 1000/2 = 500, min(500, 500) = 500
			expectedTotalScore:  sdkmath.NewInt(500),
			expectedDelta:       sdkmath.ZeroInt(), // 500 - 500 = 0
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tracker := &CostakerRewardsTracker{
				StartPeriodCumulativeReward: 1,
				ActiveSatoshis:              tc.activeSatoshis,
				ActiveBaby:                  tc.activeBaby,
				TotalScore:                  tc.initialTotalScore,
			}

			delta := tracker.UpdateScore(tc.scoreRatioBtcByBaby)

			require.Equal(t, tc.expectedTotalScore.String(), tracker.TotalScore.String(), "total score should match expected")
			require.Equal(t, tc.expectedDelta.String(), delta.String(), "delta should match expected")

			// Verify the formula: Min(ActiveSatoshis, ActiveBaby / scoreRatioBtcByBaby)
			expectedMin := sdkmath.MinInt(tc.activeSatoshis, tc.activeBaby.Quo(tc.scoreRatioBtcByBaby))
			require.Equal(t, expectedMin.String(), tracker.TotalScore.String(), "score should equal min of BTC and baby/ratio")
		})
	}
}
