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

func (h MultiFinalityHooks) BtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, sats uint64) error {
	for i := range h {
		if err := h[i].BtcDelegationActivated(ctx, fpAddr, btcDelAddr, sats); err != nil {
			return err
		}
	}
	return nil
}

func (h MultiFinalityHooks) BtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, sats uint64) error {
	for i := range h {
		if err := h[i].BtcDelegationUnbonded(ctx, fpAddr, btcDelAddr, sats); err != nil {
			return err
		}
	}
	return nil
}
