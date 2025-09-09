package types

import (
	context "context"

	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Event Hooks
// These can be utilized to communicate between a finality keeper and another
// keeper which must take particular actions when finalty providers/delegators change
// state. The second keeper must implement this interface, which then the
// finality keeper can call.

// FinalityHooks event hooks for finality btcdelegation actions
type FinalityHooks interface {
	AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error
	AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error
	AfterFpStatusChange(ctx context.Context, fpAddr sdk.AccAddress, fpSecuresBabylon bool, prevStatus, newStatus btcstktypes.FinalityProviderStatus) error
}

// combine multiple finality hooks, all hook functions are run in array sequence
var _ FinalityHooks = &MultiFinalityHooks{}

type MultiFinalityHooks []FinalityHooks

func NewMultiFinalityHooks(hooks ...FinalityHooks) MultiFinalityHooks {
	return hooks
}

func (h MultiFinalityHooks) AfterFpStatusChange(ctx context.Context, fpAddr sdk.AccAddress, fpSecuresBabylon bool, prevStatus, newStatus btcstktypes.FinalityProviderStatus) error {
	for i := range h {
		if err := h[i].AfterFpStatusChange(ctx, fpAddr, fpSecuresBabylon, prevStatus, newStatus); err != nil {
			return err
		}
	}
	return nil
}

func (h MultiFinalityHooks) AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error {
	for i := range h {
		if err := h[i].AfterBtcDelegationActivated(ctx, fpAddr, btcDelAddr, fpSecuresBabylon, sats); err != nil {
			return err
		}
	}
	return nil
}

func (h MultiFinalityHooks) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error {
	for i := range h {
		if err := h[i].AfterBtcDelegationUnbonded(ctx, fpAddr, btcDelAddr, fpSecuresBabylon, sats); err != nil {
			return err
		}
	}
	return nil
}
