package types

import (
	sdkmath "cosmossdk.io/math"
)

// UpdateScore updates the TotalScore property of the costaker rewards tracker based on the current
// values of ActiveSatoshis, ActiveBaby and the given scoreRatioBtcByBaby by parameter.
// The formula for the total score is defined as Min(total active btc staked, total active baby staked / ratio)
// It also returns the delta value difference from the (current total score - previous score).
// The returned value can be negative, meaning that the current total score is lower than the previous score.
func (c *CostakerRewardsTracker) UpdateScore(scoreRatioBtcByBaby sdkmath.Int) (deltaPreviousScore sdkmath.Int) {
	previousTotalScore := c.TotalScore

	c.TotalScore = CalculateScore(scoreRatioBtcByBaby, c.ActiveBaby, c.ActiveSatoshis)
	return c.TotalScore.Sub(previousTotalScore)
}

// CalculateScore only calculates the total score based on the func min(ActiveSats, ActiveBaby/ScoreRatioBtcByBaby)
func CalculateScore(scoreRatioBtcByBaby, activeBaby, activeSats sdkmath.Int) (totalScore sdkmath.Int) {
	activeBabyByRatio := activeBaby.Quo(scoreRatioBtcByBaby)
	return sdkmath.MinInt(activeSats, activeBabyByRatio)
}
