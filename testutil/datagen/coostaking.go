package datagen

import (
	"math/rand"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
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

func GenRandomCoostakerRewardsTracker(r *rand.Rand) types.CoostakerRewardsTracker {
	startPeriod := RandomInt(r, 1000)
	totalScore := RandomMathInt(r, 100000)

	costakerRwd := types.NewCoostakerRewardsTrackerBasic(startPeriod, totalScore)
	costakerRwd.ActiveBaby = RandomMathInt(r, 1000)
	costakerRwd.ActiveSatoshis = RandomMathInt(r, 1000)
	return costakerRwd
}
