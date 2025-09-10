package keeper

import (
	"context"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) GetHistoricalRewards(ctx context.Context, period uint64) (types.HistoricalRewards, error) {
	return k.historicalRewards.Get(ctx, period)
}

func (k Keeper) setHistoricalRewards(ctx context.Context, period uint64, histRwd types.HistoricalRewards) error {
	return k.historicalRewards.Set(ctx, period, histRwd)
}

func (k Keeper) setCostakerRewardsTracker(ctx context.Context, addr sdk.AccAddress, rwdTracker types.CostakerRewardsTracker) error {
	return k.costakerRewardsTracker.Set(ctx, addr, rwdTracker)
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

func (k Keeper) GetCostakerRewardsTrackerCheckFound(ctx context.Context, costaker sdk.AccAddress) (rwd *types.CostakerRewardsTracker, found bool, err error) {
	found, err = k.costakerRewardsTracker.Has(ctx, costaker)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	rwd, err = k.GetCostakerRewards(ctx, costaker)
	if err != nil {
		return nil, false, err
	}

	return rwd, true, nil
}

func (k Keeper) GetCostakerRewards(ctx context.Context, costaker sdk.AccAddress) (*types.CostakerRewardsTracker, error) {
	v, err := k.costakerRewardsTracker.Get(ctx, costaker)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func (k Keeper) GetCostakerRewardsOrInitialize(ctx context.Context, costaker sdk.AccAddress) (*types.CostakerRewardsTracker, error) {
	costakrRwdTracker, found, err := k.GetCostakerRewardsTrackerCheckFound(ctx, costaker)
	if err != nil {
		return nil, err
	}
	if !found {
		zeroInt := math.ZeroInt()
		currentRwd, err := k.GetCurrentRewardsInitialized(ctx)
		if err != nil {
			return nil, err
		}

		// StartPeriodCumulativeReward is correctly populated by initialization of the costaker
		rwd := types.NewCostakerRewardsTracker(currentRwd.Period-1, zeroInt, zeroInt, zeroInt)
		return &rwd, nil
	}

	return costakrRwdTracker, nil
}
