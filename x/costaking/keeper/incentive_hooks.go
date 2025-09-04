package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

var _ ictvtypes.IncentiveHooks = Hooks{}

// Wrapper struct
type Hooks struct {
	k Keeper
}

// Create new distribution hooks
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// BeforeRewardWithdraw updates the costaking reward tracker and send the reward funds from costaking to incentive module.
func (h Hooks) BeforeRewardWithdraw(ctx context.Context, sType ictvtypes.StakeholderType, addr sdk.AccAddress) error {
	if sType != ictvtypes.COSTAKER {
		return nil
	}
	return h.k.costakerModified(ctx, addr)
}
