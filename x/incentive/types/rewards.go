package types

func NewBTCDelegationRewardsTracker(startPeriod, totalSat uint64) BTCDelegationRewardsTracker {
	return BTCDelegationRewardsTracker{
		StartPeriodCumulativeRewardFP: startPeriod,
		DelegationTotalActiveSat:      totalSat,
	}
}
