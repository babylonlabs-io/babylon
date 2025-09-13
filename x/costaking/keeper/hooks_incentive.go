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

// BeforeRewardWithdraw updates the costaking reward tracker and send the reward funds from costaking to incentive module.
func (h HookIncentives) BeforeRewardWithdraw(ctx context.Context, sType ictvtypes.StakeholderType, addr sdk.AccAddress) error {
	if sType != ictvtypes.COSTAKER {
		return nil
	}
	return h.k.costakerWithdrawRewards(ctx, addr)
}
