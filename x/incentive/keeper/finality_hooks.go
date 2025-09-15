package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

var _ ftypes.FinalityHooks = Hooks{}

// Wrapper struct
type Hooks struct {
	k Keeper
}

// Create new distribution hooks
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// AfterBtcDelegationUnbonded implements the FinalityHooks interface
// It handles the unbonding of a BTC delegation by removing the staked satoshis
// from the reward tracking system
func (h Hooks) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error {
	if fpSecuresBabylon {
		// if it secures babylon it should wait until that block height is rewarded to process event tracker related events
		height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
		return h.k.AddEventBtcDelegationUnbonded(ctx, height, fpAddr, btcDelAddr, sats)
	}

	// BSNs don't need to add to the event list to be processed at some specific babylon height.
	// Should update the reward tracker structures on the spot and don't care to have the rewards
	// being distributed based on the latest voting power.
	amtSat := sdkmath.NewIntFromUint64(sats)
	return h.k.BtcDelegationUnbonded(ctx, fpAddr, btcDelAddr, amtSat)
}

// AfterBtcDelegationActivated implements the FinalityHooks interface
// It handles the activation of a BTC delegation by adding the staked satoshis
// to the reward tracking system
func (h Hooks) AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error {
	if fpSecuresBabylon {
		height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
		return h.k.AddEventBtcDelegationActivated(ctx, height, fpAddr, btcDelAddr, sats)
	}

	// BSNs don't need to add to the events, can be processed instantly
	amtSat := sdkmath.NewIntFromUint64(sats)
	return h.k.BtcDelegationActivated(ctx, fpAddr, btcDelAddr, amtSat)
}

// AfterBbnFpEntersActiveSet implements the FinalityHooks interface
func (h Hooks) AfterBbnFpEntersActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	return nil
}

// AfterBbnFpRemovedFromActiveSet implements the FinalityHooks interface
func (h Hooks) AfterBbnFpRemovedFromActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	return nil
}
