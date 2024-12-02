package types

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewBTCDelegationRewardsTracker(startPeriod uint64, totalSat sdkmath.Int) BTCDelegationRewardsTracker {
	return BTCDelegationRewardsTracker{
		StartPeriodCumulativeReward: startPeriod,
		TotalActiveSat:              totalSat,
	}
}

func NewFinalityProviderCurrentRewards(currentRewards sdk.Coins, period uint64, totalActiveSatFP sdkmath.Int) FinalityProviderCurrentRewards {
	return FinalityProviderCurrentRewards{
		CurrentRewards: currentRewards,
		Period:         period,
		TotalActiveSat: totalActiveSatFP,
	}
}

func NewFinalityProviderHistoricalRewards(cumulativeRewardsPerSat sdk.Coins) FinalityProviderHistoricalRewards {
	return FinalityProviderHistoricalRewards{
		CumulativeRewardsPerSat: cumulativeRewardsPerSat,
	}
}
