package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

func (k Keeper) UpdateAllCoostakersScore(ctx context.Context, scoreRatioBtcByBaby math.Int) error {
	totalScore := math.ZeroInt()
	err := k.IterateCoostakers(ctx, func(addr sdk.AccAddress, rwdTracker types.CoostakerRewardsTracker) error {
		rwdTracker.UpdateScore(scoreRatioBtcByBaby)
		if err := k.setCoostakerRewardsTracker(ctx, addr, rwdTracker); err != nil {
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

// IterateCoostakers iterates over all the coostakers.
func (k Keeper) IterateCoostakers(ctx context.Context, it func(addr sdk.AccAddress, rwdTracker types.CoostakerRewardsTracker) error) error {
	return k.coostakerRewardsTracker.Walk(ctx, nil, func(key []byte, value types.CoostakerRewardsTracker) (stop bool, err error) {
		addr := sdk.AccAddress(key)
		err = it(addr, value)
		if err != nil {
			return true, err
		}
		return false, nil
	})
}
