package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v2/x/incentive/types"
)

// GetOrNewRewardTrackerEvent returns a new reward tracker if it doesn't exists for that block height
func (k Keeper) GetOrNewRewardTrackerEvent(ctx context.Context, height uint64) (*types.EventsPowerUpdateAtHeight, error) {
	found, err := k.rewardTrackerEvents.Has(ctx, height)
	if err != nil {
		return nil, err
	}

	if found {
		v, err := k.rewardTrackerEvents.Get(ctx, height)
		if err != nil {
			return nil, err
		}

		return &v, nil
	}

	return &types.EventsPowerUpdateAtHeight{
		Events: make([]*types.EventPowerUpdate, 0),
	}, nil
}

// SetRewardTrackerEvent returns a new reward tracker if it doesn't exists for that block height
func (k Keeper) SetRewardTrackerEvent(ctx context.Context, height uint64, ev *types.EventsPowerUpdateAtHeight) error {
	return k.rewardTrackerEvents.Set(ctx, height, *ev)
}

// AddRewardTrackerEvent gets or create a new reward tracker event, adds the new event and store it
func (k Keeper) AddRewardTrackerEvent(ctx context.Context, height uint64, newEv types.EventPowerUpdate) error {
	rwdTrackerEvent, err := k.GetOrNewRewardTrackerEvent(ctx, height)
	if err != nil {
		return err
	}

	rwdTrackerEvent.Events = append(rwdTrackerEvent.Events, &newEv)
	return k.SetRewardTrackerEvent(ctx, height, rwdTrackerEvent)
}

// DeleteRewardTrackerEvents remove the events from the store.
func (k Keeper) DeleteRewardTrackerEvents(ctx context.Context, height uint64) error {
	return k.rewardTrackerEvents.Remove(ctx, height)
}
