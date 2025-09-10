package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AddEventBtcDelegationActivated stores an event of BTC delegation activated to be processed at a later block height
func (k Keeper) AddEventBtcDelegationActivated(ctx context.Context, height uint64, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)
	fpAddr, delAddr := fp.String(), del.String()

	newEv := types.NewEventBtcDelegationActivated(fpAddr, delAddr, amtSat)
	k.Logger(ctx).Debug(
		"add reward tracker events - activated",
		"blockHeight", height,
		"fpAddr", fpAddr,
		"btcDelAddr", delAddr,
	)

	return k.AddRewardTrackerEvent(ctx, height, newEv)
}

// AddEventBtcDelegationUnbonded stores an event of BTC delegation unbonded or withdraw to be processed at a later block height
func (k Keeper) AddEventBtcDelegationUnbonded(ctx context.Context, height uint64, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)
	fpAddr, delAddr := fp.String(), del.String()

	newEv := types.NewEventBtcDelegationUnboned(fpAddr, delAddr, amtSat)
	k.Logger(ctx).Debug(
		"add reward tracker events - unbonded",
		"blockHeight", height,
		"fpAddr", fp.String(),
		"btcDelAddr", del.String(),
	)

	return k.AddRewardTrackerEvent(ctx, height, newEv)
}

// ProcessRewardTrackerEvents process all the reward tracker events from the latest processed height + 1
// until the given block height.
func (k Keeper) ProcessRewardTrackerEvents(ctx context.Context, untilBlkHeight uint64) error {
	lastProcessedHeight, err := k.GetRewardTrackerEventLastProcessedHeight(ctx)
	if err != nil {
		return err
	}

	for blkHeight := lastProcessedHeight + 1; blkHeight <= untilBlkHeight; blkHeight++ {
		if err := k.ProcessRewardTrackerEventsAtHeight(ctx, blkHeight); err != nil {
			return err
		}
	}

	return k.SetRewardTrackerEventLastProcessedHeight(ctx, untilBlkHeight)
}

// GetRewardTrackerEventsCompiledByBtcDel compiles all the reward tracker events from the latest processed height + 1
// until the given block height without updating the store.
func (k Keeper) GetRewardTrackerEventsCompiledByBtcDel(ctx context.Context, untilBlkHeight uint64) (map[string]sdkmath.Int, error) {
	lastProcessedHeight, err := k.GetRewardTrackerEventLastProcessedHeight(ctx)
	if err != nil {
		return nil, err
	}

	satsByBtcDel := make(map[string]sdkmath.Int)
	for blkHeight := lastProcessedHeight + 1; blkHeight <= untilBlkHeight; blkHeight++ {
		evts, err := k.GetOrNewRewardTrackerEvent(ctx, blkHeight)
		if err != nil {
			return nil, err
		}

		for _, untypedEvt := range evts.Events {
			switch typedEvt := untypedEvt.Ev.(type) {
			case *types.EventPowerUpdate_BtcActivated:
				evt := typedEvt.BtcActivated
				currentSat, exists := satsByBtcDel[evt.BtcDelAddr]
				if !exists {
					currentSat = sdkmath.ZeroInt()
				}
				satsByBtcDel[evt.BtcDelAddr] = currentSat.Add(evt.TotalSat)

			case *types.EventPowerUpdate_BtcUnbonded:
				evt := typedEvt.BtcUnbonded
				currentSat, exists := satsByBtcDel[evt.BtcDelAddr]
				if !exists {
					currentSat = sdkmath.ZeroInt()
				}
				satsByBtcDel[evt.BtcDelAddr] = currentSat.Sub(evt.TotalSat)
			}
		}
	}

	return satsByBtcDel, nil
}

// ProcessRewardTrackerEventsAtHeight gets all the events for that block height, process those events updating
// the reward tracker structures and deletes all the events processed.
// Note: if there is no event at that block height it returns nil.
func (k Keeper) ProcessRewardTrackerEventsAtHeight(ctx context.Context, height uint64) error {
	evts, err := k.GetOrNewRewardTrackerEvent(ctx, height)
	if err != nil {
		return err
	}

	k.Logger(ctx).Debug(
		"processing reward tracker events",
		"blockHeight", height,
		"events", len(evts.Events),
	)

	// it must process all the events without pagination
	for _, untypedEvt := range evts.Events {
		switch typedEvt := untypedEvt.Ev.(type) {
		case *types.EventPowerUpdate_BtcActivated:
			evt := typedEvt.BtcActivated
			fp, del := sdk.MustAccAddressFromBech32(evt.FpAddr), sdk.MustAccAddressFromBech32(evt.BtcDelAddr)
			if err := k.BtcDelegationActivated(ctx, fp, del, evt.TotalSat); err != nil {
				return err
			}
		case *types.EventPowerUpdate_BtcUnbonded:
			evt := typedEvt.BtcUnbonded
			fp, del := sdk.MustAccAddressFromBech32(evt.FpAddr), sdk.MustAccAddressFromBech32(evt.BtcDelAddr)
			if err := k.BtcDelegationUnbonded(ctx, fp, del, evt.TotalSat); err != nil {
				return err
			}
		}
	}

	return k.DeleteRewardTrackerEvents(ctx, height)
}

// GetOrNewRewardTrackerEvent returns a new reward tracker if it doesn't exists for that block height
func (k Keeper) GetOrNewRewardTrackerEvent(ctx context.Context, height uint64) (*types.EventsPowerUpdateAtHeight, error) {
	found, err := k.rewardTrackerEvents.Has(ctx, height)
	if err != nil {
		return nil, err
	}

	if !found {
		return &types.EventsPowerUpdateAtHeight{
			Events: make([]*types.EventPowerUpdate, 0),
		}, nil
	}

	v, err := k.rewardTrackerEvents.Get(ctx, height)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

// GetRewardTrackerEventLastProcessedHeight returns the latest processed height of the events tracker
// Note: returns zero if not found.
func (k Keeper) GetRewardTrackerEventLastProcessedHeight(ctx context.Context) (uint64, error) {
	found, err := k.rewardTrackerEventsLastProcessedHeight.Has(ctx)
	if err != nil {
		return 0, err
	}

	if !found {
		return 0, nil
	}

	return k.rewardTrackerEventsLastProcessedHeight.Get(ctx)
}

// SetRewardTrackerEventLastProcessedHeight sets the latest processed block height of events.
func (k Keeper) SetRewardTrackerEventLastProcessedHeight(ctx context.Context, blkHeight uint64) error {
	return k.rewardTrackerEventsLastProcessedHeight.Set(ctx, blkHeight)
}

// SetRewardTrackerEvent stores the events with the provided height
func (k Keeper) SetRewardTrackerEvent(ctx context.Context, height uint64, ev *types.EventsPowerUpdateAtHeight) error {
	return k.rewardTrackerEvents.Set(ctx, height, *ev)
}

// AddRewardTrackerEvent gets or create a new reward tracker event, adds the new event and store it
func (k Keeper) AddRewardTrackerEvent(ctx context.Context, height uint64, newEv *types.EventPowerUpdate) error {
	rwdTrackerEvent, err := k.GetOrNewRewardTrackerEvent(ctx, height)
	if err != nil {
		return err
	}

	rwdTrackerEvent.Events = append(rwdTrackerEvent.Events, newEv)
	return k.SetRewardTrackerEvent(ctx, height, rwdTrackerEvent)
}

// DeleteRewardTrackerEvents remove the events from the store.
func (k Keeper) DeleteRewardTrackerEvents(ctx context.Context, height uint64) error {
	return k.rewardTrackerEvents.Remove(ctx, height)
}
