package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Event Hooks
// These can be utilized to communicate between a finality keeper and another
// keeper which must take particular actions when finalty providers/delegators change
// state. The second keeper must implement this interface, which then the
// finality keeper can call.

// FinalityHooks event hooks for finality btcdelegation actions
type FinalityHooks interface {
	AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error
	AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error
	AfterBbnFpEntersActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error
	AfterBbnFpRemovedFromActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error
}

// combine multiple finality hooks, all hook functions are run in array sequence
var _ FinalityHooks = &MultiFinalityHooks{}

type MultiFinalityHooks []FinalityHooks

func NewMultiFinalityHooks(hooks ...FinalityHooks) MultiFinalityHooks {
	return hooks
}

func (h MultiFinalityHooks) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error {
	for i := range h {
		if err := h[i].AfterBtcDelegationUnbonded(ctx, fpAddr, btcDelAddr, fpSecuresBabylon, isFpInActiveSet, sats); err != nil {
			return err
		}
	}
	return nil
}

func (h MultiFinalityHooks) AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, isFpInActiveSet bool, sats uint64) error {
	for i := range h {
		if err := h[i].AfterBtcDelegationActivated(ctx, fpAddr, btcDelAddr, fpSecuresBabylon, isFpInActiveSet, sats); err != nil {
			return err
		}
	}
	return nil
}

func (h MultiFinalityHooks) AfterBbnFpEntersActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	for i := range h {
		if err := h[i].AfterBbnFpEntersActiveSet(ctx, fpAddr); err != nil {
			return err
		}
	}
	return nil
}

func (h MultiFinalityHooks) AfterBbnFpRemovedFromActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	for i := range h {
		if err := h[i].AfterBbnFpRemovedFromActiveSet(ctx, fpAddr); err != nil {
			return err
		}
	}
	return nil
}
