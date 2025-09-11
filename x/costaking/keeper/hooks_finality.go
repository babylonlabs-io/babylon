package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

var _ ftypes.FinalityHooks = HookFinality{}

// Wrapper struct
type HookFinality struct {
	k Keeper
}

// AfterBtcDelegationActivated adds new active satoshis to the costaker reward tracker
func (h HookFinality) AfterBtcDelegationActivated(ctx context.Context, fpAddr sdk.AccAddress, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, prevStatus btcstktypes.FinalityProviderStatus, sats uint64) error {
	if !fpSecuresBabylon || !prevStatus.IsActive() {
		return nil
	}

	return h.k.costakerModified(ctx, btcDelAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.AddRaw(int64(sats))
	})
}

// AfterBtcDelegationUnbonded subtracts active satoshis to the costaker reward tracker
func (h HookFinality) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr sdk.AccAddress, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, fpBbnPrevStatus btcstktypes.FinalityProviderStatus, sats uint64) error {
	if !fpSecuresBabylon || !fpBbnPrevStatus.IsActive() {
		return nil
	}

	return h.k.costakerModified(ctx, btcDelAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.SubRaw(int64(sats))
	})
}

// AfterFpStatusChange iterates over all the delegators of this fp and reduces or increases the voting power accordingly to the fp new status
func (h HookFinality) AfterFpStatusChange(ctx context.Context, fpAddr sdk.AccAddress, fpSecuresBabylon bool, prevStatus, newStatus btcstktypes.FinalityProviderStatus) error {
	if !fpSecuresBabylon { // only fps that secure babylon should take into account
		return nil
	}

	// sanity check? should never happen
	if prevStatus == newStatus {
		return nil
	}

	// Status transition logic (not first time):
	//
	// ACTIVE   -> JAILED, SLASHED, INACTIVE : subtract voting power (-)

	// INACTIVE -> ACTIVE            : add voting power (+)
	// INACTIVE -> JAILED, SLASHED   : no action
	// JAILED   -> ACTIVE            : add voting power (+)
	// JAILED   -> INACTIVE, SLASHED : no action
	// SLASHED  -> ACTIVE            : add voting power (+)
	// SLASHED  -> JAILED, INACTIVE  : no action

	// prevStatus == (INACTIVE|JAILED|SLASHED)
	isPrevStatusDeactivated := (prevStatus == btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE ||
		prevStatus == btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_JAILED ||
		prevStatus == btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_SLASHED)

	isNewStatusActive := newStatus.IsActive()
	// (INACTIVE|JAILED|SLASHED) -> ACTIVE: add voting power
	shouldAdd := isPrevStatusDeactivated && isNewStatusActive

	// ACTIVE -> ANY: subtract voting power (shouldAdd remains false)
	shouldSubtract := prevStatus.IsActive() // reminder that can't go ACTIVE -> ACTIVE, sanity check already made

	if !shouldAdd && !shouldSubtract {
		// If no action needed, return early
		return nil
	}

	return h.k.ictvK.IterateBTCDelegationSatsUpdated(ctx, fpAddr, func(del sdk.AccAddress, activeSats math.Int) error {
		return h.k.costakerModified(ctx, del, func(rwdTracker *types.CostakerRewardsTracker) {
			if shouldAdd {
				rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.Add(activeSats)
				return
			}
			rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.Sub(activeSats)
		})
	})
}

// Create new distribution hooks
func (k Keeper) HookFinality() HookFinality {
	return HookFinality{k}
}
