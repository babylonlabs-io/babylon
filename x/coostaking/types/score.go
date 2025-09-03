package types

import "cosmossdk.io/math"

func (c *CoostakerRewardsTracker) UpdateScore(scoreRatioBtcByBaby math.Int) {
	// dummy logic
	// TODO replace with real score update logic
	c.TotalScore = c.ActiveSatoshis.Mul(scoreRatioBtcByBaby).Quo(math.NewInt(100))
}
