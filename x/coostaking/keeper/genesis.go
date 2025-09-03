package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

// InitGenesis performs stateful validations and initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	for _, rwdTrackerEntry := range gs.CoostakersRewardsTracker {
		coostakerAddr, err := sdk.AccAddressFromBech32(rwdTrackerEntry.CoostakerAddress)
		if err != nil {
			return err
		}

		err = k.setCoostakerRewardsTracker(ctx, coostakerAddr, *rwdTrackerEntry.Tracker)
		if err != nil {
			return err
		}
	}

	for _, histRwd := range gs.HistoricalRewards {
		err := k.setHistoricalRewards(ctx, histRwd.Period, *histRwd.Rewards)
		if err != nil {
			return err
		}
	}

	if gs.CurrentRewards.Rewards != nil {
		if err := k.SetCurrentRewards(ctx, *gs.CurrentRewards.Rewards); err != nil {
			return err
		}
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	historicalRewards, err := k.getHistoricalRewardsEntries(ctx)
	if err != nil {
		return nil, err
	}

	coostakersRewardsTracker, err := k.getCoostakerRewardsTrackerEntries(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:                   k.GetParams(ctx),
		CurrentRewards:           k.getCurrentRewardsEntry(ctx),
		HistoricalRewards:        historicalRewards,
		CoostakersRewardsTracker: coostakersRewardsTracker,
	}, nil
}

// getCurrentRewardsEntry gets current rewards as genesis entry.
func (k Keeper) getCurrentRewardsEntry(ctx context.Context) types.CurrentRewardsEntry {
	currentRewards, err := k.GetCurrentRewards(ctx)
	if err != nil {
		// If no current rewards are found, return empty entry
		return types.CurrentRewardsEntry{}
	}

	return types.CurrentRewardsEntry{Rewards: currentRewards}
}

// getHistoricalRewardsEntries gets all historical rewards stored.
// This function has high resource consumption and should only be used on export genesis.
func (k Keeper) getHistoricalRewardsEntries(ctx context.Context) ([]types.HistoricalRewardsEntry, error) {
	entries := make([]types.HistoricalRewardsEntry, 0)

	iter, err := k.historicalRewards.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		period, err := iter.Key()
		if err != nil {
			return nil, err
		}

		rewards, err := iter.Value()
		if err != nil {
			return nil, err
		}

		entry := types.HistoricalRewardsEntry{
			Period:  period,
			Rewards: &rewards,
		}

		if err := entry.Validate(); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// getCoostakerRewardsTrackerEntries gets all coostaker rewards trackers stored.
// This function has high resource consumption and should only be used on export genesis.
func (k Keeper) getCoostakerRewardsTrackerEntries(ctx context.Context) ([]types.CoostakerRewardsTrackerEntry, error) {
	entries := make([]types.CoostakerRewardsTrackerEntry, 0)

	iter, err := k.coostakerRewardsTracker.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, err
		}

		tracker, err := iter.Value()
		if err != nil {
			return nil, err
		}

		addr := sdk.AccAddress(key)
		entry := types.CoostakerRewardsTrackerEntry{
			CoostakerAddress: addr.String(),
			Tracker:          &tracker,
		}

		if err := entry.Validate(); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
