package datagen

import (
	"math/rand"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func GenRandomCurrentRewards(r *rand.Rand) types.CurrentRewards {
	rewards := GenRandomCoins(r)
	period := RandomInt(r, 1000)
	totalScore := RandomMathInt(r, 100000)
	return types.NewCurrentRewards(rewards, period, totalScore)
}

func GenRandomHistoricalRewards(r *rand.Rand) types.HistoricalRewards {
	cumulativeRewardsPerScore := GenRandomCoins(r)
	return types.NewHistoricalRewards(cumulativeRewardsPerScore)
}

func GenRandomCostakerRewardsTracker(r *rand.Rand) types.CostakerRewardsTracker {
	startPeriod := RandomInt(r, 1000)
	totalScore := RandomMathInt(r, 100000)

	costakerRwd := types.NewCostakerRewardsTrackerBasic(startPeriod, totalScore)
	costakerRwd.ActiveBaby = RandomMathInt(r, 1000)
	costakerRwd.ActiveSatoshis = RandomMathInt(r, 1000)
	return costakerRwd
}
