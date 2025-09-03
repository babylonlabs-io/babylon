package keeper

import (
	"context"

	"cosmossdk.io/math"
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

// coostakerModifiedActiveAmounts anytime an coostaker changes his amount of btc or baby staked this function
// should be called, for activation of new staking or unbonding of the previous, his score might change and then it should
// also update the total score of the pool of current rewards
func (k Keeper) coostakerModifiedActiveAmounts(ctx context.Context, coostaker sdk.AccAddress, newActiveSatoshi, newActiveBaby math.Int) error {
	rwdTracker, err := k.GetCoostakerRewardsOrInitialize(ctx, coostaker)
	if err != nil {
		return err
	}

	rwdTracker.ActiveBaby = newActiveBaby
	rwdTracker.ActiveSatoshis = newActiveSatoshi

	params := k.GetParams(ctx)
	deltaScoreChange := rwdTracker.UpdateScore(params.ScoreRatioBtcByBaby)

	if deltaScoreChange.IsZero() {
		// if there is no change from previous score, just update the coostaker tracker and return
		return k.setCoostakerRewardsTracker(ctx, coostaker, *rwdTracker)
	}

	// if there is change on the score, calls the coostaker modified score and set the updated tracker
	// Note: the coostaker tracker must be updated after incrementing the period and calculating the rewards
	return k.coostakerModifiedScoreWithPreInitalization(ctx, coostaker, func(ctx context.Context, coostaker sdk.AccAddress) error {
		// Save the tracker back to storage since ActiveSatoshis/ActiveBaby changed
		err = k.setCoostakerRewardsTracker(ctx, coostaker, *rwdTracker)
		if err != nil {
			return err
		}

		// updates the rewards pool total score active
		curRwd, err := k.GetCurrentRewards(ctx)
		if err != nil {
			return err
		}

		// Note: delta score change here can be negative and will reduce the total score
		curRwd.TotalScore = curRwd.TotalScore.Add(deltaScoreChange)
		return k.SetCurrentRewards(ctx, *curRwd)
	})
}

func (k Keeper) coostakerModified(ctx context.Context, coostaker sdk.AccAddress) error {
	return k.coostakerModifiedScoreWithPreInitalization(ctx, coostaker, func(ctx context.Context, coostaker sdk.AccAddress) error {
		return nil
	})
}

// coostakerModifiedScoreWithPreInitalization does the procedure when a Coostaker has
// some modification in its total amount of score (btc or baby staked). This function
// increments the global current rewards period (that creates a new historical) with
// the ended period, calculates the coostaker reward and send to the gauge
// and calls a function prior (preInitializeCoostaker) to initialize a new
// Coostaker tracker, which is useful to apply subtract or add the total
// amount of score by the coostaker.
func (k Keeper) coostakerModifiedScoreWithPreInitalization(
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

	return k.initializeCoStakerRwdTracker(ctx, coostaker)
}

// IncrementRewardsPeriod finalizes the current reward period and starts a new one.
// It does the following:
//   - Computes the per-score rewards for the ending period and adds them to the
//     cumulative historical rewards.
//   - Stores the updated historical rewards for the ended period.
//   - Initializes a new empty reward period with the same total score.
//
// Note: Rewards in historical entries are stored with extra decimal precision
// (DecimalRewards). They must be reduced to standard precision in
// CalculateCoStakerRewards before distribution.
func (k Keeper) IncrementRewardsPeriod(ctx context.Context) (endedPeriod uint64, err error) {
	// IncrementPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per score)
	//    sets new empty current rewards with new period
	currentRwd, err := k.GetCurrentRewardsInitialized(ctx)
	if err != nil {
		return 0, err
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
	return k.bankK.SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, rewards)
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

	return k.calculateCoStakerRewardsBetween(ctx, *coostakerRwdTracker, endPeriod)
}

// calculateCoStakerRewardsBetween computes the rewards accrued by a coostaker
// between two periods: the tracker’s StartPeriodCumulativeReward (inclusive)
// and the provided endingPeriod (inclusive).
//
// It derives rewards from the cumulative per-score amounts:
//
//	ΔPerScore = Historical[endingPeriod].CumulativeRewardsPerScore
//	          − Historical[StartPeriod].CumulativeRewardsPerScore
//
// The coostaker’s gross rewards (with decimals) are then:
//
//	RewardsWithDecimals = ΔPerScore * tracker.TotalScore
//
// Finally, rewards are scaled back to standard precision by dividing by
// ictvtypes.DecimalRewards (truncating), yielding sdk.Coins to distribute.
func (k Keeper) calculateCoStakerRewardsBetween(
	ctx context.Context,
	coostakerRwdTracker types.CoostakerRewardsTracker,
	endingPeriod uint64,
) (sdk.Coins, error) {
	if coostakerRwdTracker.StartPeriodCumulativeReward > endingPeriod {
		return sdk.Coins{}, types.ErrInvalidPeriod.Wrapf("startingPeriod %d cannot be greater than endingPeriod %d", coostakerRwdTracker.StartPeriodCumulativeReward, endingPeriod)
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
	differenceWithDecimals, isNegative := ending.CumulativeRewardsPerScore.SafeSub(starting.CumulativeRewardsPerScore...)
	if isNegative {
		return sdk.Coins{}, types.ErrNegativeRewards.Wrapf("cumulative rewards is negative %s", differenceWithDecimals.String())
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

// initializeCoStakerRwdTracker initializes a CoostakerRewardsTracker for the given
// coostaker using the cumulative rewards at the end of the *previous* period.
// This should be called immediately after the coostaker’s rewards are withdrawn
// (i.e., sent to the reward gauge), or after any change to the coostaker’s
// staking amount (sat or baby), which requires withdrawing rewards and resetting
// the tracker.
//
// Precondition: the global rewards period has already been incremented before
// calling this function. The tracker’s start period is set to (current.Period - 1),
// so subsequent accruals are computed from that point forward while preserving
// the coostaker’s current TotalScore.
func (k Keeper) initializeCoStakerRwdTracker(ctx context.Context, coostaker sdk.AccAddress) error {
	currentRwd, err := k.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}
	// The global period has been incremented before this call.
	// Use the ended period as the tracker’s starting point for future accruals.
	previousPeriod := currentRwd.Period - 1

	coostakerRwdTracker, err := k.GetCoostakerRewards(ctx, coostaker)
	if err != nil {
		return err
	}

	coostakerRwdTracker.StartPeriodCumulativeReward = previousPeriod
	return k.setCoostakerRewardsTracker(ctx, coostaker, *coostakerRwdTracker)
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
