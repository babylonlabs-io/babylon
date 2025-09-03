package types

import (
	sdkmath "cosmossdk.io/math"
)

// UpdateScore updates the TotalScore property of the coostaker rewards tracker based on the current
// values of ActiveSatoshis, ActiveBaby and the given scoreRatioBtcByBaby by parameter.
// The formula for the total score is defined as Min(total active btc staked, total active baby staked / ratio)
// It also returns the delta value difference from the (current total score - previous score).
// The returned value can be negative, meaning that the current total score is lower than the previous score.
func (c *CoostakerRewardsTracker) UpdateScore(scoreRatioBtcByBaby sdkmath.Int) (deltaPreviousScore sdkmath.Int) {
	previousTotalScore := c.TotalScore

	activeBabyByRatio := c.ActiveBaby.Quo(scoreRatioBtcByBaby)
	c.TotalScore = sdkmath.MinInt(c.ActiveSatoshis, activeBabyByRatio)

	return c.TotalScore.Sub(previousTotalScore)
}
