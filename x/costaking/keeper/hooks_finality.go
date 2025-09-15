package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

var _ ftypes.FinalityHooks = HookFinality{}

// Wrapper struct
type HookFinality struct {
	k Keeper
}

// AfterBtcDelegationUnbonded subtracts active satoshis from the costaker reward tracker
func (h HookFinality) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr sdk.AccAddress, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error {
	if !fpSecuresBabylon || !isFpInActiveSet {
		return nil
	}

	return h.k.costakerModified(ctx, btcDelAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.SubRaw(int64(sats))
	})
}

// AfterBtcDelegationActivated adds new active satoshis to the costaker reward tracker
func (h HookFinality) AfterBtcDelegationActivated(ctx context.Context, fpAddr sdk.AccAddress, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error {
	if !fpSecuresBabylon || !isFpInActiveSet {
		return nil
	}

	return h.k.costakerModified(ctx, btcDelAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.AddRaw(int64(sats))
	})
}

// AfterBbnFpEntersActiveSet iterates over all the delegators of this fp and adds their voting power
func (h HookFinality) AfterBbnFpEntersActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	return h.k.ictvK.IterateBTCDelegationSatsUpdated(ctx, fpAddr, func(del sdk.AccAddress, activeSats math.Int) error {
		return h.k.costakerModified(ctx, del, func(rwdTracker *types.CostakerRewardsTracker) {
			rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.Add(activeSats)
		})
	})
}

// AfterBbnFpRemovedFromActiveSet iterates over all the delegators of this fp and subtracts their voting power
func (h HookFinality) AfterBbnFpRemovedFromActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	return h.k.ictvK.IterateBTCDelegationSatsUpdated(ctx, fpAddr, func(del sdk.AccAddress, activeSats math.Int) error {
		return h.k.costakerModified(ctx, del, func(rwdTracker *types.CostakerRewardsTracker) {
			rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.Sub(activeSats)
		})
	})
}

// Create new distribution hooks
func (k Keeper) HookFinality() HookFinality {
	return HookFinality{k}
}
