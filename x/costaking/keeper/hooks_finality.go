package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

var _ ftypes.FinalityHooks = HookFinality{}

// Wrapper struct
type HookFinality struct {
	k Keeper
}

// AfterBtcDelegationActivated adds new active satoshis to the costaker reward tracker
func (h HookFinality) AfterBtcDelegationActivated(ctx context.Context, fpAddr sdk.AccAddress, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error {
	return h.k.costakerModified(ctx, btcDelAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.AddRaw(int64(sats))
	})
}

// AfterBtcDelegationUnbonded subtracts active satoshis to the costaker reward tracker
func (h HookFinality) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr sdk.AccAddress, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error {
	return h.k.costakerModified(ctx, btcDelAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveSatoshis = rwdTracker.ActiveSatoshis.SubRaw(int64(sats))
	})
}

// AfterFpStatusChange iterates over all the delegators of this fp and reduces or increases the voting power accordingly to the fp new status
func (h HookFinality) AfterFpStatusChange(ctx context.Context, fpAddr sdk.AccAddress, fpSecuresBabylon bool, newStatus btcstktypes.FinalityProviderStatus) error {
	return h.k.ictvK.IterateBTCDelegationRewardsTracker(ctx, fpAddr, func(fp, del sdk.AccAddress, value ictvtypes.BTCDelegationRewardsTracker) error {

		// CASE: first time fp is active
		// what happens the first time? FP was inactive at first and received btc delegation, AfterBtcDelegationActivated is called for this
		// btc delegation and later the AfterFpStatusChange

		return h.k.costakerModified(ctx, del, func(rwdTracker *types.CostakerRewardsTracker) {
			// not first time
			if newStatus == btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE {
				rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Add(value.TotalActiveSat)
				return
			}

			rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Sub(value.TotalActiveSat)
		})
	})
}

// Create new distribution hooks
func (k Keeper) HookFinality() HookFinality {
	return HookFinality{k}
}
