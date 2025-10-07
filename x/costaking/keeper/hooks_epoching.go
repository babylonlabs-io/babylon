package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

var _ epochingtypes.EpochingHooks = HookEpoching{}

// Wrapper struct
type HookEpoching struct {
	k Keeper
}

// AfterEpochBegins is called after an epoch begins
func (h HookEpoching) AfterEpochBegins(ctx context.Context, epoch uint64) {
	// store the current validator set with their original tokens
	if err := h.k.updateValidatorSet(ctx); err != nil {
		h.k.Logger(ctx).Error("failed to store validator set", "error", err)
	}
}

// AfterEpochEnds is called after an epoch ends
// It handles the transition of validators between active and inactive states:
// - Newly active validators: add their delegators' baby tokens to ActiveBaby
// - Newly inactive validators: remove their delegators' baby tokens from ActiveBaby
func (h HookEpoching) AfterEpochEnds(ctx context.Context, epoch uint64) {
	// Get the validator set from the ending epoch (cached in stkCache)
	prevValMap, err := h.k.stkCache.GetActiveValidatorSet(ctx, h.k.buildCurrEpochValSetMap)
	if err != nil {
		h.k.Logger(ctx).Error("failed to get previous validator set", "error", err)
		return
	}

	// Build the new validator set map from the staking module
	// Note: This is called after ApplyAndReturnValidatorSetUpdates, so the staking
	// module's last validator powers reflect the NEW epoch's validator set
	newValMap, err := h.buildNewActiveValSetMap(ctx)
	if err != nil {
		h.k.Logger(ctx).Error("failed to build new validator set", "error", err)
		return
	}

	// Identify newly active validators (in new set but not in prev set)
	for valAddr := range newValMap {
		if _, found := prevValMap[valAddr]; !found {
			// Newly active validator - add baby tokens for all delegators
			if err := h.addBabyForDelegators(ctx, valAddr); err != nil {
				h.k.Logger(ctx).Error("failed to add baby tokens for newly active validator", "validator", valAddr, "error", err)
				return
			}
		}
	}

	// Identify newly inactive validators (in prev set but not in new set)
	for prevValAddr, prevVal := range prevValMap {
		if _, found := newValMap[prevValAddr]; !found {
			// Newly inactive validator - remove baby tokens for all delegators
			if err := h.removeBabyForDelegators(ctx, prevVal); err != nil {
				h.k.Logger(ctx).Error("failed to remove baby tokens for newly inactive validator", "validator", prevValAddr, "error", err)
				return
			}
		}
	}
}

// updateCoStkTrackerForDelegators updates costaking tracker for all delegators of a validator
func (h HookEpoching) updateCoStkTrackerForDelegators(
	ctx context.Context,
	val stakingtypes.ValidatorI,
	updateFn func(*types.CostakerRewardsTracker, math.Int),
) error {
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	if err != nil {
		return err
	}

	delegations, err := h.k.stkK.GetValidatorDelegations(ctx, valAddr)
	if err != nil {
		return err
	}

	for _, del := range delegations {
		delAddr := sdk.MustAccAddressFromBech32(del.DelegatorAddress)

		// Get delegation tokens
		delTokens := val.TokensFromShares(del.Shares)

		// Update ActiveBaby using the provided update function
		if err := h.k.costakerModified(ctx, delAddr, func(rwdTracker *types.CostakerRewardsTracker) {
			updateFn(rwdTracker, delTokens.TruncateInt())
		}); err != nil {
			h.k.Logger(ctx).Error("failed to update costaker tracker",
				"delegator", delAddr.String(),
				"error", err)
			return err
		}
	}

	return nil
}

// addBabyForDelegators adds baby tokens to all delegators of a newly active validator
func (h HookEpoching) addBabyForDelegators(ctx context.Context, valAddrStr string) error {
	val, err := h.k.stkK.GetValidator(ctx, sdk.ValAddress(valAddrStr))
	if err != nil {
		return fmt.Errorf("failed to get validator %s: %w", valAddrStr, err)
	}
	return h.updateCoStkTrackerForDelegators(ctx, val, func(rwdTracker *types.CostakerRewardsTracker, amount math.Int) {
		rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Add(amount)
	})
}

// removeBabyForDelegators removes baby tokens from all delegators of a newly inactive validator
func (h HookEpoching) removeBabyForDelegators(ctx context.Context, valInfo types.ValidatorInfo) error {
	// Get validator from staking keeper to get updated shares
	val, err := h.k.stkK.GetValidator(ctx, valInfo.ValAddress)
	if err != nil {
		return fmt.Errorf("failed to get validator %s: %w", valInfo.ValAddress.String(), err)
	}
	if valInfo.IsSlashed {
		// If the validator has been slashed, we need to restore the original tokens
		// before removing the baby tokens to avoid miscalculating the token amount
		val.Tokens = valInfo.OriginalTokens
	}
	return h.updateCoStkTrackerForDelegators(ctx, val, func(rwdTracker *types.CostakerRewardsTracker, amount math.Int) {
		rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Sub(amount)
	})
}

// BeforeSlashThreshold is called before a certain threshold of validators are slashed
func (h HookEpoching) BeforeSlashThreshold(ctx context.Context, valSet epochingtypes.ValidatorSet) {
}

// buildNewActiveValSetMap builds the new active validator set map
// from the staking module's last validator powers (for next epoch)
func (h HookEpoching) buildNewActiveValSetMap(ctx context.Context) (map[string]struct{}, error) {
	valMap := make(map[string]struct{})

	err := h.k.stkK.IterateLastValidatorPowers(ctx, func(valAddr sdk.ValAddress, power int64) bool {
		valMap[valAddr.String()] = struct{}{}
		return false // continue iteration
	})

	if err != nil {
		return nil, err
	}

	return valMap, nil
}

// Create new epoching hooks
func (k Keeper) HookEpoching() HookEpoching {
	return HookEpoching{k}
}
