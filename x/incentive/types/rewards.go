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

func (f *FinalityProviderCurrentRewards) AddRewards(coinsToAdd sdk.Coins) {
	f.CurrentRewards = f.CurrentRewards.Add(coinsToAdd...)
}

func (f *FinalityProviderCurrentRewards) SubRewards(coinsToSubtract sdk.Coins) {
	f.CurrentRewards = f.CurrentRewards.Sub(coinsToSubtract...)
}

func (f *FinalityProviderCurrentRewards) AddTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Add(amt)
}

func (f *FinalityProviderCurrentRewards) SubTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Sub(amt)
}

func (f *BTCDelegationRewardsTracker) AddTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Add(amt)
}

func (f *BTCDelegationRewardsTracker) SubTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Sub(amt)
}
