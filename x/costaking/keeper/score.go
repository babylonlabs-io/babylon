package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func (k Keeper) UpdateAllCostakersScore(ctx context.Context, scoreRatioBtcByBaby math.Int) error {
	totalScore := math.ZeroInt()
	err := k.IterateCostakers(ctx, func(addr sdk.AccAddress, rwdTracker types.CostakerRewardsTracker) error {
		rwdTracker.UpdateScore(scoreRatioBtcByBaby)
		if err := k.setCostakerRewardsTracker(ctx, addr, rwdTracker); err != nil {
			return err
		}

		totalScore = totalScore.Add(rwdTracker.TotalScore)
		return nil
	})
	if err != nil {
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
