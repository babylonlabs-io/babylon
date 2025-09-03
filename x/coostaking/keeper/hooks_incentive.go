package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

var _ ictvtypes.IncentiveHooks = HookIncentives{}

// Wrapper struct
type HookIncentives struct {
	k Keeper
}

// Create new distribution hooks
func (k Keeper) HookIncentives() HookIncentives {
	return HookIncentives{k}
}

// BeforeRewardWithdraw updates the coostaking reward tracker and send the reward funds from coostaking to incentive module.
func (h HookIncentives) BeforeRewardWithdraw(ctx context.Context, sType ictvtypes.StakeholderType, addr sdk.AccAddress) error {
	if sType != ictvtypes.COOSTAKER {
		return nil
	}
	return h.k.coostakerWithdrawRewards(ctx, addr)
}
