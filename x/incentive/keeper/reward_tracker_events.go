package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v2/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AddEventBtcDelegationActivated stores an event of BTC delegation activated to be processed at a later block height
func (k Keeper) AddEventBtcDelegationActivated(ctx context.Context, height uint64, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)
	newEv := types.NewEventBtcDelegationActivated(fp.String(), del.String(), amtSat)
	return k.AddRewardTrackerEvent(ctx, height, newEv)
}

// AddEventBtcDelegationUnbonded stores an event of BTC delegation unbonded or withdraw to be processed at a later block height
func (k Keeper) AddEventBtcDelegationUnbonded(ctx context.Context, height uint64, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)
	newEv := types.NewEventBtcDelegationUnboned(fp.String(), del.String(), amtSat)
	return k.AddRewardTrackerEvent(ctx, height, newEv)
}

// ProcessRewardTrackerEvents gets all the events for that block height, process those events updating
// the reward tracker structures and deletes all the events processed.
// Note: if there is no event at that block height it returns nil.
func (k Keeper) ProcessRewardTrackerEvents(ctx context.Context, height uint64) error {
	evts, err := k.GetOrNewRewardTrackerEvent(ctx, height)
	if err != nil {
		return err
	}

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
		case *types.EventPowerUpdate_SlashedFp:
			// do nothing for now
		}
	}

	return k.DeleteRewardTrackerEvents(ctx, height)
}
