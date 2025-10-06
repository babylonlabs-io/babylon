package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func (k Keeper) UpdateAllCostakersScore(ctx context.Context, scoreRatioBtcByBaby math.Int) error {
	totalScore := math.ZeroInt()

	endedPeriod, err := k.IncrementRewardsPeriod(ctx)
	if err != nil {
		return err
	}

	currentRwd, err := k.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}

	err = k.IterateCostakers(ctx, func(costaker sdk.AccAddress, rwdTracker types.CostakerRewardsTracker) error {
		deltaScoreChange := rwdTracker.UpdateScore(scoreRatioBtcByBaby)

		totalScore = totalScore.Add(rwdTracker.TotalScore)
		if deltaScoreChange.IsZero() {
			// if there is no change from previous score, continue
			return k.setCostakerRewardsTracker(ctx, costaker, rwdTracker)
		}

		if err := k.CalculateCostakerRewardsAndSendToGauge(ctx, costaker, endedPeriod); err != nil {
			return err
		}

		if err := k.setCostakerRewardsTracker(ctx, costaker, rwdTracker); err != nil {
			return err
		}

		return k.initializeCoStakerRwdTracker(ctx, costaker)
	})
	if err != nil {
		return err
	}

	currentRwd.TotalScore = totalScore
	if err := currentRwd.Validate(); err != nil {
		return err
	}

	return k.UpdateCurrentRewardsTotalScore(ctx, totalScore)
}

// IterateCostakers iterates over all the costakers.
func (k Keeper) IterateCostakers(ctx context.Context, it func(addr sdk.AccAddress, rwdTracker types.CostakerRewardsTracker) error) error {
	return k.costakerRewardsTracker.Walk(ctx, nil, func(key []byte, value types.CostakerRewardsTracker) (stop bool, err error) {
		addr := sdk.AccAddress(key)
		err = it(addr, value)
		if err != nil {
			return true, err
		}
		return false, nil
	})
}
