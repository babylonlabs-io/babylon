package keeper

import (
	"context"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) GetHistoricalRewards(ctx context.Context, period uint64) (types.HistoricalRewards, error) {
	return k.historicalRewards.Get(ctx, period)
}

func (k Keeper) setHistoricalRewards(ctx context.Context, period uint64, histRwd types.HistoricalRewards) error {
	return k.historicalRewards.Set(ctx, period, histRwd)
}

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
	v, err := k.currentRewards.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func (k Keeper) GetCurrentRewardsCheckFound(ctx context.Context) (rwd *types.CurrentRewards, found bool, err error) {
	found, err = k.currentRewards.Has(ctx)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	rwd, err = k.GetCurrentRewards(ctx)
	if err != nil {
		return nil, false, err
	}

	return rwd, true, nil
}

func (k Keeper) GetCoostakerRewardsTrackerCheckFound(ctx context.Context, coostaker sdk.AccAddress) (rwd *types.CoostakerRewardsTracker, found bool, err error) {
	found, err = k.coostakerRewardsTracker.Has(ctx, coostaker)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	rwd, err = k.GetCoostakerRewards(ctx, coostaker)
	if err != nil {
		return nil, false, err
	}

	return rwd, true, nil
}

func (k Keeper) GetCoostakerRewards(ctx context.Context, coostaker sdk.AccAddress) (*types.CoostakerRewardsTracker, error) {
	v, err := k.coostakerRewardsTracker.Get(ctx, coostaker)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func (k Keeper) GetCoostakerRewardsOrInitialize(ctx context.Context, coostaker sdk.AccAddress) (*types.CoostakerRewardsTracker, error) {
	coostakerRwdTracker, found, err := k.GetCoostakerRewardsTrackerCheckFound(ctx, coostaker)
	if err != nil {
		return nil, err
	}
	if !found {
		zeroInt := math.ZeroInt()
		// StartPeriodCumulativeReward is correctly populated by initialization of the coostaker
		rwd := types.NewCoostakerRewardsTracker(0, zeroInt, zeroInt, zeroInt)
		return &rwd, nil
	}

	return coostakerRwdTracker, nil
}
