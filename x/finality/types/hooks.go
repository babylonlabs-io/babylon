package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// combine multiple staking hooks, all hook functions are run in array sequence
var _ FinalityHooks = &MultiFinalityHooks{}

type MultiFinalityHooks []FinalityHooks

func NewMultiFinalityHooks(hooks ...FinalityHooks) MultiFinalityHooks {
	return hooks
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
