package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Event Hooks
// These can be utilized to communicate between a incentive keeper and another
// keeper which must take particular actions when actors withdraw their rewards.
// The keeper must implement this interface, which then the incentive keeper can call.

// IncentiveHooks event hooks for incentive btcdelegation actions
type IncentiveHooks interface {
	BeforeRewardWithdraw(ctx context.Context, sType StakeholderType, addr sdk.AccAddress) error
}

// combine multiple incentive hooks, all hook functions are run in array sequence
var _ IncentiveHooks = &MultiIncentiveHooks{}

type MultiIncentiveHooks []IncentiveHooks

func NewMultiIncentiveHooks(hooks ...IncentiveHooks) MultiIncentiveHooks {
	return hooks
}

func (h MultiIncentiveHooks) BeforeRewardWithdraw(ctx context.Context, sType StakeholderType, addr sdk.AccAddress) error {
	for i := range h {
		if err := h[i].BeforeRewardWithdraw(ctx, sType, addr); err != nil {
			return err
		}
	}
	return nil
}
