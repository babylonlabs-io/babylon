package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

var _ stktypes.StakingHooks = HookStaking{}

// Wrapper struct
type HookStaking struct {
	k Keeper
}

// AfterDelegationModified handles Baby token delegation modification events.
// This hook is triggered when an existing cosmos staking delegation amount is changed
// (increased or decreased). It updates the costaker's Baby token amount accordingly.
//
// State Changes:
// - ActiveBaby += (new_amount - old_amount)
// - If differece is negative, ActiveBaby is subtracted
//
// Note: This hook uses a cache to track previous delegation amounts to calculate the delta.
func (h HookStaking) AfterDelegationModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	// Check if validator is in the active set
	valSet, err := h.k.stkCache.GetActiveValidatorSet(ctx, h.k.buildCurrEpochValSetMap)
	if err != nil {
		return err
	}

	// NOTE: co-staking genesis is called before staking genesis.
	// The active set will be populated during the staking genesis but after calling the hooks, so the active validators map will be empty.
	// Thus, for testing purposes, we assume all validators are active if the block height is 0.
	assumeActiveValidatorIfGenesis(ctx, valSet, valAddr)

	if _, ok := valSet[valAddr.String()]; !ok {
		// Validator not in active set, skip processing
		return nil
	}

	del, err := h.k.stkK.GetDelegation(ctx, delAddr, valAddr)
	if err != nil { // we stop if the delegation is not found, because it must be found
		return err
	}

	delTokens, err := h.k.TokensFromShares(ctx, valAddr, del.Shares)
	if err != nil {
		return err
	}

	delTokensBefore := h.k.stkCache.GetStakedAmount(delAddr, valAddr)
	delTokenChange := delTokens.Sub(delTokensBefore).TruncateInt()
	return h.k.costakerModified(ctx, delAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Add(delTokenChange)
	})
}

// BeforeDelegationSharesModified handles pre-modification state caching.
// This hook is triggered before a cosmos staking delegation is modified. It caches
// the current delegation amount so that AfterDelegationModified can calculate the delta.
//
// State Changes:
// - Caches current delegation amount in temporary storage
func (h HookStaking) BeforeDelegationSharesModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	// Check if validator is in the active set
	valSet, err := h.k.stkCache.GetActiveValidatorSet(ctx, h.k.buildCurrEpochValSetMap)
	if err != nil {
		return err
	}

	// NOTE: co-staking genesis is called before staking genesis.
	// The active set will be populated during the staking genesis but after calling the hooks, so the active validators map will be empty.
	// Thus, for testing purposes, we assume all validators are active if the set is empty and block height is 0.
	assumeActiveValidatorIfGenesis(ctx, valSet, valAddr)

	if _, ok := valSet[valAddr.String()]; !ok {
		// Validator not in active set, skip processing
		return nil
	}

	del, err := h.k.stkK.GetDelegation(ctx, delAddr, valAddr)
	if err != nil {
		// probably is not found, but we don't want to stop execution for this
		h.k.Logger(ctx).Error("hook costaking BeforeDelegationSharesModified", err)
		return nil
	}

	delTokens, err := h.k.TokensFromShares(ctx, valAddr, del.Shares)
	if err != nil {
		return err
	}
	h.k.stkCache.SetStakedAmount(delAddr, valAddr, delTokens)
	return nil
}

// AfterUnbondingInitiated implements types.StakingHooks.
func (h HookStaking) AfterUnbondingInitiated(ctx context.Context, id uint64) error {
	return nil
}

// AfterValidatorBeginUnbonding implements types.StakingHooks.
func (h HookStaking) AfterValidatorBeginUnbonding(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	return nil
}

// AfterValidatorBonded implements types.StakingHooks.
func (h HookStaking) AfterValidatorBonded(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	return nil
}

// AfterValidatorCreated implements types.StakingHooks.
func (h HookStaking) AfterValidatorCreated(ctx context.Context, valAddr sdk.ValAddress) error {
	return nil
}

// AfterValidatorRemoved implements types.StakingHooks.
func (h HookStaking) AfterValidatorRemoved(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	return nil
}

// BeforeDelegationCreated implements types.StakingHooks.
func (h HookStaking) BeforeDelegationCreated(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

// BeforeDelegationRemoved implements types.StakingHooks.
func (h HookStaking) BeforeDelegationRemoved(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

// BeforeValidatorModified implements types.StakingHooks.
func (h HookStaking) BeforeValidatorModified(ctx context.Context, valAddr sdk.ValAddress) error {
	return nil
}

// BeforeValidatorSlashed implements types.StakingHooks.
func (h HookStaking) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction math.LegacyDec) error {
	return nil
}

// Create new staking hooks
func (k Keeper) HookStaking() HookStaking {
	return HookStaking{k}
}

// TokensFromShares gets the validator and returns the tokens based on the amount of shares
func (k Keeper) TokensFromShares(ctx context.Context, valAddr sdk.ValAddress, delShares math.LegacyDec) (math.LegacyDec, error) {
	valI, err := k.stkK.Validator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}
	delTokens := valI.TokensFromShares(delShares)
	return delTokens, nil
}

// buildCurrEpochValSetMap builds the current epoch's validator set map
// from the epoching module
func (k Keeper) buildCurrEpochValSetMap(ctx context.Context) (map[string]struct{}, error) {
	valMap := make(map[string]struct{})

	// Get the current epoch's validator set from the epoching keeper
	valSet := k.epochingK.GetCurrentValidatorSet(ctx)

	// Convert epoching ValidatorSet to map
	for _, val := range valSet {
		valAddr := sdk.ValAddress(val.Addr)
		valMap[valAddr.String()] = struct{}{}
	}

	return valMap, nil
}

// assumeActiveValidatorIfGenesis adds the given validator to the active set if block height is genesis height (0)
// and the validator is not already in the set
func assumeActiveValidatorIfGenesis(ctx context.Context, valSet map[string]struct{}, valAddr sdk.ValAddress) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if sdkCtx.BlockHeader().Height == 0 {
		// Add validator to active set during genesis
		valSet[valAddr.String()] = struct{}{}
	}
}
