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

// BeforeRewardWithdraw handles costaker reward withdrawal preparation.
// This hook is triggered before the incentive module processes a reward withdrawal.
// For costakers, it calculates and transfers the appropriate reward amounts from the
// costaking module account to the incentive module account before distribution.
//
// Note: This is also being used in a query context to calculate the total rewards available.
//
// State Changes:
// - Updates costaker's reward period tracking
// - Transfers calculated rewards from costaking module to incentive module
// - Only processes COSTAKER stakeholder type, ignores others
func (h HookIncentives) BeforeRewardWithdraw(ctx context.Context, sType ictvtypes.StakeholderType, addr sdk.AccAddress) error {
	if sType != ictvtypes.COSTAKER {
		return nil
	}
	return h.k.costakerWithdrawRewards(ctx, addr)
}
