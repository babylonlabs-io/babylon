package types

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// it is needed to add decimal points when reducing the rewards amount
	// per sat to latter when giving out the rewards to the gauge, reduce
	// the decimal points back, currently 20 decimal points are being added
	// the sdkmath.Int holds a big int which support up to 2^256 integers
	DecimalAccumulatedRewards, _ = sdkmath.NewIntFromString("100000000000000000000")
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
