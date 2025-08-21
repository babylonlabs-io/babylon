package keeper

import (
	"context"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) setCoostakerRewardsTracker(ctx context.Context, addr sdk.AccAddress, rwdTracker types.CoostakerRewardsTracker) error {
	return k.coostakerRewardsTracker.Set(ctx, addr, rwdTracker)
}

func (k Keeper) setCurrentRewards(ctx context.Context, currentRwd types.CurrentRewards) error {
	return k.currentRewards.Set(ctx, currentRwd)
}

func (k Keeper) UpdateCurrentRewardsTotalScore(ctx context.Context, totalScore math.Int) error {
	currentRwd, err := k.currentRewards.Get(ctx)
	if err != nil {
		return err
	}

	currentRwd.TotalScore = totalScore
	// TODO(rafilx): initialize a new period, creates historical...
	return k.setCurrentRewards(ctx, currentRwd)
}
