package keeper

import (
	"context"

	"cosmossdk.io/math"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

var _ epochingtypes.EpochingHooks = HookEpoching{}

// Wrapper struct
type HookEpoching struct {
	k Keeper
}

// AfterEpochBegins is called after an epoch begins
func (h HookEpoching) AfterEpochBegins(ctx context.Context, epoch uint64) {
}

// AfterEpochEnds is called after an epoch ends
// It handles the transition of validators between active and inactive states:
// - Newly active validators: add their delegators' baby tokens to ActiveBaby
// - Newly inactive validators: remove their delegators' baby tokens from ActiveBaby
func (h HookEpoching) AfterEpochEnds(ctx context.Context, epoch uint64) {
	// Get the validator set from the ending epoch (cached in stkCache)
	prevValSet := h.k.stkCache.GetValidatorSet(ctx, h.k.epochingK)

	// Build the new validator set map from the staking module
	// Note: This is called after ApplyAndReturnValidatorSetUpdates, so the staking
	// module's last validator powers reflect the NEW epoch's validator set
	newValMap, err := h.buildNewValSetMap(ctx)
	if err != nil {
		h.k.Logger(ctx).Error("failed to build new validator set", "error", err)
		return
	}

	// Build map for previous validator set
	prevValMap := make(map[string]bool)
	for _, val := range prevValSet {
		prevValMap[val.GetValAddress().String()] = true
	}

	// Identify newly active validators (in new set but not in prev set)
	for valAddr := range newValMap {
		if !prevValMap[valAddr] {
			// Newly active validator - add baby tokens for all delegators
			if err := h.addBabyForDelegators(ctx, valAddr); err != nil {
				h.k.Logger(ctx).Error("failed to add baby tokens for newly active validator", "validator", valAddr, "error", err)
			}
		}
	}

	// Identify newly inactive validators (in prev set but not in new set)
	for _, val := range prevValSet {
		valAddr := val.GetValAddress()
		valAddrStr := valAddr.String()
		if !newValMap[valAddrStr] {
			// Newly inactive validator - remove baby tokens for all delegators
			if err := h.removeBabyForDelegators(ctx, valAddrStr); err != nil {
				h.k.Logger(ctx).Error("failed to remove baby tokens for newly inactive validator", "validator", valAddrStr, "error", err)
			}
		}
	}
}

// buildNewValSetMap builds the new validator set from the staking module's last validator powers
// Returns both a map for efficient lookups and a slice for iteration
func (h HookEpoching) buildNewValSetMap(ctx context.Context) (map[string]bool, error) {
	newValMap := make(map[string]bool)

	err := h.k.stkK.IterateLastValidatorPowers(ctx, func(valAddr sdk.ValAddress, power int64) bool {
		newValMap[valAddr.String()] = true
		return false // continue iteration
	})

	if err != nil {
		return nil, err
	}

	return newValMap, nil
}

// updateCoStkTrackerForDelegators updates costaking tracker for all delegators of a validator
func (h HookEpoching) updateCoStkTrackerForDelegators(
	ctx context.Context,
	valAddrStr string,
	updateFn func(*types.CostakerRewardsTracker, math.Int),
) error {
	valAddr, err := sdk.ValAddressFromBech32(valAddrStr)
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
		delTokens, err := h.k.TokensFromShares(ctx, valAddr, del.Shares)
		if err != nil {
			h.k.Logger(ctx).Error("failed to convert shares to tokens",
				"delegator", delAddr.String(),
				"validator", valAddrStr,
				"error", err)
			continue
		}

		// Update ActiveBaby using the provided update function
		if err := h.k.costakerModified(ctx, delAddr, func(rwdTracker *types.CostakerRewardsTracker) {
			updateFn(rwdTracker, delTokens.TruncateInt())
		}); err != nil {
			h.k.Logger(ctx).Error("failed to update costaker tracker",
				"delegator", delAddr.String(),
				"error", err)
		}
	}

	return nil
}

// addBabyForDelegators adds baby tokens to all delegators of a newly active validator
func (h HookEpoching) addBabyForDelegators(ctx context.Context, valAddrStr string) error {
	return h.updateCoStkTrackerForDelegators(ctx, valAddrStr, func(rwdTracker *types.CostakerRewardsTracker, amount math.Int) {
		rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Add(amount)
	})
}

// removeBabyForDelegators removes baby tokens from all delegators of a newly inactive validator
func (h HookEpoching) removeBabyForDelegators(ctx context.Context, valAddrStr string) error {
	return h.updateCoStkTrackerForDelegators(ctx, valAddrStr, func(rwdTracker *types.CostakerRewardsTracker, amount math.Int) {
		rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Sub(amount)
	})
}

// BeforeSlashThreshold is called before a certain threshold of validators are slashed
func (h HookEpoching) BeforeSlashThreshold(ctx context.Context, valSet epochingtypes.ValidatorSet) {
}

// Create new epoching hooks
func (k Keeper) HookEpoching() HookEpoching {
	return HookEpoching{k}
}
