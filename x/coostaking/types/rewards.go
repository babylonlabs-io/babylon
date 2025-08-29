package types

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

func NewCoostakerRewardsTracker(startPeriod uint64, totalScore sdkmath.Int) CoostakerRewardsTracker {
	return CoostakerRewardsTracker{
		StartPeriodCumulativeReward: startPeriod,
		TotalScore:                  totalScore,
	}
}

func NewCurrentRewards(currentRewards sdk.Coins, period uint64, totalScore sdkmath.Int) CurrentRewards {
	return CurrentRewards{
		Rewards:    currentRewards,
		Period:     period,
		TotalScore: totalScore,
	}
}

func NewHistoricalRewards(cumulativeRewardsPerScore sdk.Coins) HistoricalRewards {
	return HistoricalRewards{
		CumulativeRewardsPerScore: cumulativeRewardsPerScore,
	}
}

// AddRewards adds the rewards to the CurrentRewards and applies the decimal
// multiplier to increase precision for calculating rewards per active satoshi staked
func (f *CurrentRewards) AddRewards(coinsToAdd sdk.Coins) error {
	coinsToAddWithDecimals, err := bbntypes.CoinsSafeMulInt(coinsToAdd, ictvtypes.DecimalRewards)
	if err != nil {
		return err
	}

	var panicErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = ictvtypes.ErrInvalidAmount.Wrapf("math overflow: %v", r)
			}
		}()
		f.Rewards = f.Rewards.Add(coinsToAddWithDecimals...)
	}()
	return panicErr
}
