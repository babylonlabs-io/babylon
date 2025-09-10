package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
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

// AfterBtcDelegationActivated implements the FinalityHooks interface
// It handles the activation of a BTC delegation by adding the staked satoshis
// to the reward tracking system
func (h Hooks) AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, fpBbnPrevStatus btcstktypes.FinalityProviderStatus, sats uint64) error {
	if fpSecuresBabylon {
		height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
		return h.k.AddEventBtcDelegationActivated(ctx, height, fpAddr, btcDelAddr, sats)
	}

	// BSNs don't need to add to the events, can be processed instantly
	amtSat := sdkmath.NewIntFromUint64(sats)
	return h.k.BtcDelegationActivated(ctx, fpAddr, btcDelAddr, amtSat)
}

// AfterBtcDelegationUnbonded implements the FinalityHooks interface
// It handles the unbonding of a BTC delegation by removing the staked satoshis
// from the reward tracking system
func (h Hooks) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, fpBbnPrevStatus btcstktypes.FinalityProviderStatus, sats uint64) error {
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

// AfterFpStatusChange implements types.FinalityHooks
func (h Hooks) AfterFpStatusChange(ctx context.Context, fpAddr sdk.AccAddress, fpSecuresBabylon bool, prevStatus, newStatus types.FinalityProviderStatus) error {
	return nil
}
