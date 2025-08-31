package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// AddRewardsForCoostakers gets the current coostaker pool of rewards
// and adds rewards to it, without increasing the current period.
func (k Keeper) AddRewardsForCoostakers(ctx context.Context, rwd sdk.Coins) error {
	currentRwd, err := k.GetCurrentRewardsInitialized(ctx)
	if err != nil {
		return err
	}

	if err := currentRwd.AddRewards(rwd); err != nil {
		return err
	}
	return k.SetCurrentRewards(ctx, *currentRwd)
}

// coostakerModifiedWithPreInitalization does the procedure when a Coostaker has
// some modification in its total amount of score (btc or baby staked). This function
// increments the global current rewards period (that creates a new historical) with
// the ended period, calculates the coostaker reward and send to the gauge
// and calls a function prior (preInitializeCoostaker) to initialize a new
// Coostaker tracker, which is useful to apply subtract or add the total
// amount of score by the coostaker.
func (k Keeper) coostakerModifiedWithPreInitalization(
	ctx context.Context,
	coostaker sdk.AccAddress,
	preInitializeCoostaker func(ctx context.Context, coostaker sdk.AccAddress) error,
) error {
	endedPeriod, err := k.IncrementRewardsPeriod(ctx)
	if err != nil {
		return err
	}

	if err := k.CalculateCoostakerRewardsAndSendToGauge(ctx, coostaker, endedPeriod); err != nil {
		return err
	}

	if err := preInitializeCoostaker(ctx, coostaker); err != nil {
		return err
	}

	return k.initializeCoostakerRwdTracker(ctx, coostaker)
}

// IncrementRewardsPeriod gets or initializes the current rewards structure,
// increases the period from the current rewards and empty the rewards.
// It also creates a new historical with the ended period and sets the rewards
// of the newly historical period as the amount from the previous historical
// plus the amount of rewards that each score staked is entitled to receive.
// The rewards in the historical are stored with multiplied decimals
// (DecimalRewards) to increase precision, and need to be
// reduced when the rewards are calculated in CalculateCoostakerRewards
// prior to send out to the coostaker gauge.
func (k Keeper) IncrementRewardsPeriod(ctx context.Context) (endedPeriod uint64, err error) {
	// IncrementPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per score)
	//    sets new empty current rewards with new period
	currentRwd, err := k.GetCurrentRewardsInitialized(ctx)
	if err != nil {
		return 0, err
	}
	if currentRwd.Period == 1 {
		// first time, no need to calculate tokens or set historical again
		return 1, nil
	}

	currentRewardsPerScore := sdk.NewCoins()
	if !currentRwd.TotalScore.IsZero() {
		// the current rewards is already set with decimals
		currentRewardsPerScoreWithDecimals := currentRwd.Rewards
		currentRewardsPerScore = currentRewardsPerScoreWithDecimals.QuoInt(currentRwd.TotalScore)
	}

	historicalRwd, err := k.GetHistoricalRewards(ctx, currentRwd.Period-1)
	if err != nil {
		return 0, err
	}

	newHistoricalRwd := types.NewHistoricalRewards(historicalRwd.CumulativeRewardsPerScore.Add(currentRewardsPerScore...))
	if err := k.setHistoricalRewards(ctx, currentRwd.Period, newHistoricalRwd); err != nil {
		return 0, err
	}

	// initiates a new period with empty rewards and the same amount of active sat
	newCurrentRwd := types.NewCurrentRewards(sdk.NewCoins(), currentRwd.Period+1, currentRwd.TotalScore)
	if err := k.SetCurrentRewards(ctx, newCurrentRwd); err != nil {
		return 0, err
	}

	return currentRwd.Period, nil
}

// CalculateCoostakerRewardsAndSendToGauge calculates the rewards of the coostaker based on the
// StartPeriodCumulativeReward and the received endPeriod and sends to the coostaker gauge.
func (k Keeper) CalculateCoostakerRewardsAndSendToGauge(ctx context.Context, coostaker sdk.AccAddress, endPeriod uint64) error {
	rewards, err := k.CalculateCoostakerRewards(ctx, coostaker, endPeriod)
	if err != nil {
		return err
	}

	if rewards.IsZero() {
		return nil
	}

	k.ictvK.AccumulateRewardGaugeForCoostaker(ctx, coostaker, rewards)
	return nil
}

// CalculateCoostakerRewards calculates the rewards entitled for this coostaker
// from the starting period cumulative reward and the ending period received as parameter
// It returns the amount of rewards without decimals (it removes the DecimalRewards).
func (k Keeper) CalculateCoostakerRewards(ctx context.Context, coostaker sdk.AccAddress, endPeriod uint64) (sdk.Coins, error) {
	coostakerRwdTracker, found, err := k.GetCoostakerRewardsTrackerCheckFound(ctx, coostaker)
	if err != nil {
		return sdk.Coins{}, err
	}
	if !found {
		return sdk.NewCoins(), nil
	}

	if coostakerRwdTracker.TotalScore.IsZero() {
		return sdk.NewCoins(), nil
	}

	return k.calculateCoostakerRewardsBetween(ctx, *coostakerRwdTracker, endPeriod)
}

// calculateCoostakerRewardsBetween calculate the rewards accured of a coostaker between
// two period, the endingPeriod received in param and the StartPeriodCumulativeReward of
// the CoostakerRewardsTracker. It gets the CumulativeRewardsPerScore of the ending
// period and subtracts the CumulativeRewardsPerScore of the starting period
// that give the total amount of rewards that one score is entitle to receive
// in rewards between those two period. To get the amount this coostaker should
// receive, it multiplies by the total amount of active score this coostaker has.
// One note, before give out the rewards it quotes by the DecimalRewards
// to get it ready to be sent out to the delegator reward gauge.
func (k Keeper) calculateCoostakerRewardsBetween(
	ctx context.Context,
	coostakerRwdTracker types.CoostakerRewardsTracker,
	endingPeriod uint64,
) (sdk.Coins, error) {
	if coostakerRwdTracker.StartPeriodCumulativeReward > endingPeriod {
		panic("startingPeriod cannot be greater than endingPeriod")
	}

	// return staking * (ending - starting)
	starting, err := k.GetHistoricalRewards(ctx, coostakerRwdTracker.StartPeriodCumulativeReward)
	if err != nil {
		return sdk.Coins{}, err
	}

	ending, err := k.GetHistoricalRewards(ctx, endingPeriod)
	if err != nil {
		return sdk.Coins{}, err
	}

	// creates the differenceWithDecimals amount of rewards (ending - starting) periods
	// this differenceWithDecimals is the amount of rewards entitled per score
	differenceWithDecimals := ending.CumulativeRewardsPerScore.Sub(starting.CumulativeRewardsPerScore...)
	if differenceWithDecimals.IsAnyNegative() {
		panic("negative rewards should not be possible")
	}

	rewardsWithDecimals, err := bbntypes.CoinsSafeMulInt(differenceWithDecimals, coostakerRwdTracker.TotalScore)
	if err != nil {
		return sdk.Coins{}, err
	}

	// note: necessary to truncate so we don't allow withdrawing more rewardsWithDecimals than owed
	// QuoInt already truncates
	rewards := rewardsWithDecimals.QuoInt(ictvtypes.DecimalRewards)
	return rewards, nil
}

// initializeCoostakerRwdTracker creates a new CoostakerRewardsTracker from the
// previous acumulative rewards period of the global pool reward tracker. This function
// should be called right after a coostaker withdraw his rewards (in our
// case send the rewards to the reward gauge). Reminder that at every new
// modification to the amount of satoshi staked or baby staked from this delegator, it
// should withdraw all rewards (send to gauge) and initialize a new CoostakerRewardsTracker.
func (k Keeper) initializeCoostakerRwdTracker(ctx context.Context, coostaker sdk.AccAddress) error {
	// period has already been incremented prior to call this function
	// it is needed to store the period ended by this delegation action
	// as a starting point of the delegation rewards calculation
	currentRwd, err := k.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}
	previousPeriod := currentRwd.Period - 1

	coostakerRwdTracker, err := k.GetCoostakerRewards(ctx, coostaker)
	if err != nil {
		return err
	}

	rwd := types.NewCoostakerRewardsTracker(previousPeriod, coostakerRwdTracker.TotalScore)
	return k.setCoostakerRewardsTracker(ctx, coostaker, rwd)
}

// GetCurrentRewardsInitialized returns the current period, if it is not found it initializes it.
func (k Keeper) GetCurrentRewardsInitialized(ctx context.Context) (rwd *types.CurrentRewards, err error) {
	// IncrementPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per score)
	//    sets new empty current rewards with new period
	currentRwd, found, err := k.GetCurrentRewardsCheckFound(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		// initialize reward tracking system and set the period as 1 as ended period due
		// to the created historical FP rewards starts at period 0
		currentRwd, err = k.initializeRewardsTracker(ctx)
		if err != nil {
			return nil, err
		}
	}

	return currentRwd, nil
}

// initializeRewardsTracker initializes a new current rewards tracker at period 1, empty rewards and zero score
// and also creates a new historical rewards at period 0 and zero rewards as well.
// It does not verifies if it exists prior to overwrite, who calls it needs to verify.
func (k Keeper) initializeRewardsTracker(ctx context.Context) (*types.CurrentRewards, error) {
	// historical rewards starts at the period 0
	err := k.setHistoricalRewards(ctx, 0, types.NewHistoricalRewards(sdk.NewCoins()))
	if err != nil {
		return nil, err
	}

	// set current rewards (starting at period 1)
	curRwd := types.NewCurrentRewards(sdk.NewCoins(), 1, sdkmath.ZeroInt())
	return &curRwd, k.SetCurrentRewards(ctx, curRwd)
}
