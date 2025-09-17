package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// AddRewardsForCostakers gets the current costaker pool of rewards
// and adds rewards to it, without increasing the current period.
func (k Keeper) AddRewardsForCostakers(ctx context.Context, rwdToAdd sdk.Coins) error {
	currentRwd, err := k.GetCurrentRewardsInitialized(ctx)
	if err != nil {
		return err
	}

	k.EmitEventCostakersAddRewards(ctx, rwdToAdd, *currentRwd)
	if err := currentRwd.AddRewards(rwdToAdd); err != nil {
		return err
	}
	return k.SetCurrentRewards(ctx, *currentRwd)
}

func (k Keeper) costakerModified(ctx context.Context, costaker sdk.AccAddress, modifyCostaker func(rwdTracker *types.CostakerRewardsTracker)) error {
	rwdTracker, err := k.GetCostakerRewardsOrInitialize(ctx, costaker)
	if err != nil {
		return err
	}

	modifyCostaker(rwdTracker)
	if err := rwdTracker.Validate(); err != nil {
		return err
	}

	params := k.GetParams(ctx)
	deltaScoreChange := rwdTracker.UpdateScore(params.ScoreRatioBtcByBaby)

	if deltaScoreChange.IsZero() {
		// if there is no change from previous score, just update the costaker tracker and return
		return k.setCostakerRewardsTracker(ctx, costaker, *rwdTracker)
	}

	// if there is change on the score, calls the costaker modified score and set the updated tracker
	// Note: the costaker tracker must be updated after incrementing the period and calculating the rewards
	return k.costakerModifiedScoreWithPreInitalization(ctx, costaker, func(ctx context.Context, costaker sdk.AccAddress) error {
		// Save the tracker back to storage since ActiveSatoshis/ActiveBaby changed
		err = k.setCostakerRewardsTracker(ctx, costaker, *rwdTracker)
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
		if err := curRwd.Validate(); err != nil {
			return err
		}
		return k.SetCurrentRewards(ctx, *curRwd)
	})
}

// costakerModifiedActiveAmounts anytime an costaker changes his amount of btc or baby staked this function
// should be called, for activation of new staking or unbonding of the previous, his score might change and then it should
// also update the total score of the pool of current rewards
func (k Keeper) costakerModifiedActiveAmounts(ctx context.Context, costaker sdk.AccAddress, newActiveSatoshi, newActiveBaby math.Int) error {
	return k.costakerModified(ctx, costaker, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveBaby = newActiveBaby
		rwdTracker.ActiveSatoshis = newActiveSatoshi
	})
}

// costakerWithdrawRewards even though the costaker didn't modified the total score, since he withdraw the rewards
// there is a need to increase his period and the global rewards pool period as well
func (k Keeper) costakerWithdrawRewards(ctx context.Context, costaker sdk.AccAddress) error {
	return k.costakerModifiedScoreWithPreInitalization(ctx, costaker, func(ctx context.Context, costaker sdk.AccAddress) error {
		return nil
	})
}

// costakerModifiedScoreWithPreInitalization does the procedure when a Costaker has
// some modification in its total amount of score (btc or baby staked). This function
// increments the global current rewards period (that creates a new historical) with
// the ended period, calculates the costaker reward and send to the gauge
// and calls a function prior (preInitializeCostaker) to initialize a new
// Costaker tracker, which is useful to apply subtract or add the total
// amount of score by the costaker.
func (k Keeper) costakerModifiedScoreWithPreInitalization(
	ctx context.Context,
	costaker sdk.AccAddress,
	preInitializeCostaker func(ctx context.Context, costaker sdk.AccAddress) error,
) error {
	endedPeriod, err := k.IncrementRewardsPeriod(ctx)
	if err != nil {
		return err
	}

	if err := k.CalculateCostakerRewardsAndSendToGauge(ctx, costaker, endedPeriod); err != nil {
		return err
	}

	if err := preInitializeCostaker(ctx, costaker); err != nil {
		return err
	}

	return k.initializeCoStakerRwdTracker(ctx, costaker)
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

// CalculateCostakerRewardsAndSendToGauge calculates the rewards of the costaker based on the
// StartPeriodCumulativeReward and the received endPeriod and sends to the costaker gauge.
func (k Keeper) CalculateCostakerRewardsAndSendToGauge(ctx context.Context, costaker sdk.AccAddress, endPeriod uint64) error {
	rewards, err := k.CalculateCostakerRewards(ctx, costaker, endPeriod)
	if err != nil {
		return err
	}

	if rewards.IsZero() {
		return nil
	}

	k.ictvK.AccumulateRewardGaugeForCostaker(ctx, costaker, rewards)
	return k.bankK.SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, rewards)
}

// CalculateCostakerRewards calculates the rewards entitled for this costaker
// from the starting period cumulative reward and the ending period received as parameter
// It returns the amount of rewards without decimals (it removes the DecimalRewards).
func (k Keeper) CalculateCostakerRewards(ctx context.Context, costaker sdk.AccAddress, endPeriod uint64) (sdk.Coins, error) {
	costakerRwdTracker, found, err := k.GetCostakerRewardsTrackerCheckFound(ctx, costaker)
	if err != nil {
		return sdk.Coins{}, err
	}
	if !found {
		return sdk.NewCoins(), nil
	}

	if costakerRwdTracker.TotalScore.IsZero() {
		return sdk.NewCoins(), nil
	}

	return k.calculateCoStakerRewardsBetween(ctx, *costakerRwdTracker, endPeriod)
}

// calculateCoStakerRewardsBetween computes the rewards accrued by a costaker
// between two periods: the tracker’s StartPeriodCumulativeReward (inclusive)
// and the provided endingPeriod (inclusive).
//
// It derives rewards from the cumulative per-score amounts:
//
//	ΔPerScore = Historical[endingPeriod].CumulativeRewardsPerScore
//	          − Historical[StartPeriod].CumulativeRewardsPerScore
//
// The costaker’s gross rewards (with decimals) are then:
//
//	RewardsWithDecimals = ΔPerScore * tracker.TotalScore
//
// Finally, rewards are scaled back to standard precision by dividing by
// ictvtypes.DecimalRewards (truncating), yielding sdk.Coins to distribute.
func (k Keeper) calculateCoStakerRewardsBetween(
	ctx context.Context,
	costakerRwdTracker types.CostakerRewardsTracker,
	endingPeriod uint64,
) (sdk.Coins, error) {
	if costakerRwdTracker.StartPeriodCumulativeReward > endingPeriod {
		return sdk.Coins{}, types.ErrInvalidPeriod.Wrapf("startingPeriod %d cannot be greater than endingPeriod %d", costakerRwdTracker.StartPeriodCumulativeReward, endingPeriod)
	}

	// return staking * (ending - starting)
	starting, err := k.GetHistoricalRewards(ctx, costakerRwdTracker.StartPeriodCumulativeReward)
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

	rewardsWithDecimals, err := bbntypes.CoinsSafeMulInt(differenceWithDecimals, costakerRwdTracker.TotalScore)
	if err != nil {
		return sdk.Coins{}, err
	}

	// note: necessary to truncate so we don't allow withdrawing more rewardsWithDecimals than owed
	// QuoInt already truncates
	rewards := rewardsWithDecimals.QuoInt(ictvtypes.DecimalRewards)
	return rewards, nil
}

// initializeCoStakerRwdTracker initializes a CostakerRewardsTracker for the given
// costaker using the cumulative rewards at the end of the *previous* period.
// This should be called immediately after the costaker’s rewards are withdrawn
// (i.e., sent to the reward gauge), or after any change to the costaker’s
// staking amount (sat or baby), which requires withdrawing rewards and resetting
// the tracker.
//
// Precondition: the global rewards period has already been incremented before
// calling this function. The tracker’s start period is set to (current.Period - 1),
// so subsequent accruals are computed from that point forward while preserving
// the costaker’s current TotalScore.
func (k Keeper) initializeCoStakerRwdTracker(ctx context.Context, costaker sdk.AccAddress) error {
	currentRwd, err := k.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}
	// The global period has been incremented before this call.
	// Use the ended period as the tracker’s starting point for future accruals.
	previousPeriod := currentRwd.Period - 1

	costakerRwdTracker, err := k.GetCostakerRewards(ctx, costaker)
	if err != nil {
		return err
	}

	costakerRwdTracker.StartPeriodCumulativeReward = previousPeriod
	return k.setCostakerRewardsTracker(ctx, costaker, *costakerRwdTracker)
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
