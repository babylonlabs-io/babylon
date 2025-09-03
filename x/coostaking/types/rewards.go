package types

import (
	"fmt"

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
// multiplier to increase precision for calculating rewards per score
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

// Validate validates the CurrentRewards struct
func (cr CurrentRewards) Validate() error {
	if !cr.Rewards.IsValid() {
		return fmt.Errorf("invalid rewards: %s", cr.Rewards.String())
	}
	if cr.Rewards.IsAnyNegative() {
		return fmt.Errorf("rewards cannot be negative: %s", cr.Rewards.String())
	}
	if cr.TotalScore.IsNil() || cr.TotalScore.IsNegative() {
		return fmt.Errorf("total score must be non-negative: %s", cr.TotalScore.String())
	}
	return nil
}

// Validate validates the HistoricalRewards struct
func (hr HistoricalRewards) Validate() error {
	if !hr.CumulativeRewardsPerScore.IsValid() {
		return fmt.Errorf("invalid cumulative rewards per score: %s", hr.CumulativeRewardsPerScore.String())
	}
	if hr.CumulativeRewardsPerScore.IsAnyNegative() {
		return fmt.Errorf("cumulative rewards per score cannot be negative: %s", hr.CumulativeRewardsPerScore.String())
	}
	return nil
}

// Validate validates the CoostakerRewardsTracker struct
func (crt CoostakerRewardsTracker) Validate() error {
	if crt.TotalScore.IsNil() || crt.TotalScore.IsNegative() {
		return fmt.Errorf("total score must be non-negative: %s", crt.TotalScore.String())
	}
	return nil
}
