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

func (k Keeper) SetCurrentRewards(ctx context.Context, currentRwd types.CurrentRewards) error {
	return k.currentRewards.Set(ctx, currentRwd)
}

func (k Keeper) UpdateCurrentRewardsTotalScore(ctx context.Context, totalScore math.Int) error {
	currentRwd, err := k.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}

	currentRwd.TotalScore = totalScore
	// TODO(rafilx): initialize a new period, creates historical...
	return k.SetCurrentRewards(ctx, *currentRwd)
}

func (k Keeper) GetCurrentRewards(ctx context.Context) (*types.CurrentRewards, error) {
	found, err := k.currentRewards.Has(ctx)
	if err != nil {
		return nil, err
	}

	if !found {
		return &types.CurrentRewards{
			Rewards:    sdk.NewCoins(),
			Period:     1,
			TotalScore: math.ZeroInt(),
		}, nil
	}

	v, err := k.currentRewards.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func (k Keeper) AddCurrentRewards(ctx context.Context, newRewards sdk.Coins) error {
	currentRwd, err := k.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}

	currentRwd.Rewards = currentRwd.Rewards.Add(newRewards...)
	return k.SetCurrentRewards(ctx, *currentRwd)
}
