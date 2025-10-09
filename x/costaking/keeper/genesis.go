package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

// InitGenesis performs stateful validations and initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	for _, rwdTrackerEntry := range gs.CostakersRewardsTracker {
		costakerAddr, err := sdk.AccAddressFromBech32(rwdTrackerEntry.CostakerAddress)
		if err != nil {
			return err
		}

		err = k.setCostakerRewardsTracker(ctx, costakerAddr, *rwdTrackerEntry.Tracker)
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

	if err := k.initValidatorSet(ctx, gs); err != nil {
		return err
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	historicalRewards, err := k.getHistoricalRewardsEntries(ctx)
	if err != nil {
		return nil, err
	}

	costakersRewardsTracker, err := k.getCostakerRewardsTrackerEntries(ctx)
	if err != nil {
		return nil, err
	}

	valSet, err := k.validatorSet.Get(ctx)
	if err != nil {
		// If the key is empty, will return an error. Log the error and return empty validator set.
		k.Logger(ctx).Error("failed to get validator set from store during export genesis", "error", err)
		valSet = types.ValidatorSet{} // return empty validator set on error
	}

	return &types.GenesisState{
		Params:                  k.GetParams(ctx),
		CurrentRewards:          k.getCurrentRewardsEntry(ctx),
		HistoricalRewards:       historicalRewards,
		CostakersRewardsTracker: costakersRewardsTracker,
		ValidatorSet:            valSet,
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

// getCostakerRewardsTrackerEntries gets all costaker rewards trackers stored.
// This function has high resource consumption and should only be used on export genesis.
func (k Keeper) getCostakerRewardsTrackerEntries(ctx context.Context) ([]types.CostakerRewardsTrackerEntry, error) {
	entries := make([]types.CostakerRewardsTrackerEntry, 0)

	iter, err := k.costakerRewardsTracker.Iterate(ctx, nil)
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
		entry := types.CostakerRewardsTrackerEntry{
			CostakerAddress: addr.String(),
			Tracker:         &tracker,
		}

		if err := entry.Validate(); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (k Keeper) initValidatorSet(ctx context.Context, gs types.GenesisState) error {
	if len(gs.ValidatorSet.Validators) > 0 {
		if err := k.validatorSet.Set(ctx, gs.ValidatorSet); err != nil {
			return err
		}
		return nil
	}
	// If empty validator set is provided, try to initialize it from the current staking validators
	valAddrs := make([]sdk.ValAddress, 0)
	if err := k.stkK.IterateLastValidatorPowers(ctx, func(valAddr sdk.ValAddress, power int64) bool {
		valAddrs = append(valAddrs, valAddr)
		return false // continue iteration
	}); err != nil {
		if err := k.validatorSet.Set(ctx, gs.ValidatorSet); err != nil {
			return err
		}
	}

	if err := k.updateValidatorSet(ctx, valAddrs); err != nil {
		if err := k.validatorSet.Set(ctx, gs.ValidatorSet); err != nil {
			return err
		}
	}
	return nil
}
