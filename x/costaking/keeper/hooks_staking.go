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
	if err := h.k.assumeActiveValidatorIfGenesis(ctx, valSet, valAddr); err != nil {
		return err
	}

	valInfo, ok := valSet[valAddr.String()]
	if !ok {
		// Validator not in active set, skip processing
		return nil
	}

	del, err := h.k.stkK.GetDelegation(ctx, delAddr, valAddr)
	if err != nil { // we stop if the delegation is not found, because it must be found
		return err
	}

	delTokens, err := h.k.TokensFromShares(ctx, valInfo, del.Shares)
	if err != nil {
		return err
	}

	delTokensBefore := h.k.stkCache.GetStakedAmount(delAddr, valAddr)
	delTokenChange := delTokens.Sub(delTokensBefore).TruncateInt()

	// In the case of unstaking to a slashed validator,
	// to properly account for the slashing, we need to ensure
	// the original tokens are updated based on the reduced tokens
	// to avoid duplicated reductions.
	if delTokenChange.IsNegative() && valInfo.IsSlashed {
		updatedOriginalTokens := valInfo.OriginalTokens.Add(delTokenChange) // delTokenChange is negative
		h.k.stkCache.UpdateOriginalTokens(valInfo.ValAddress, updatedOriginalTokens)
	}

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
	if err := h.k.assumeActiveValidatorIfGenesis(ctx, valSet, valAddr); err != nil {
		return err
	}

	valInfo, ok := valSet[valAddr.String()]
	if !ok {
		// Validator not in active set, skip processing
		return nil
	}

	del, err := h.k.stkK.GetDelegation(ctx, delAddr, valAddr)
	if err != nil {
		// probably is not found, but we don't want to stop execution for this
		h.k.Logger(ctx).Error("hook costaking BeforeDelegationSharesModified", err)
		return nil
	}

	delTokens, err := h.k.TokensFromShares(ctx, valInfo, del.Shares)
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
// This function uses the validator's original tokens stored in the module state
// to calculate the delegation tokens from shares. In this way, we avoid issues
// that may arise from changes in the validator's tokens due to slashing
func (k Keeper) TokensFromShares(ctx context.Context, valInfo types.ValidatorInfo, delShares math.LegacyDec) (math.LegacyDec, error) {
	val, err := k.stkK.GetValidator(ctx, valInfo.ValAddress)
	if err != nil {
		return math.LegacyDec{}, err
	}
	// Consider the case where the validator has been slashed.
	// The AddValidatorTokensAndShares is called before the AfterDelegationModified, so we need to correct the tokens
	// so the reduced amount in the epoching hook is the same as the one in the staking hook
	// The RemoveValidatorTokensAndShares is called after the AfterDelegationModified, so in unstaking case delTokens should be 0
	if valInfo.IsSlashed {
		delTokens := val.Tokens.Sub(valInfo.CurrentTokens)
		// update the tracked current tokens to avoid
		// double counting if multiple delegations to the same validator
		k.stkCache.UpdateCurrentTokens(valInfo.ValAddress, val.Tokens)
		// Restore the original tokens before slashing for token from share calculation
		// and update the original tokens in the validator info
		val.Tokens = valInfo.OriginalTokens.Add(delTokens)
		k.stkCache.UpdateOriginalTokens(valInfo.ValAddress, val.Tokens)
		if delTokens.IsPositive() {
			return delTokens.ToLegacyDec(), nil
		}
	}
	delTokens := val.TokensFromShares(delShares)
	return delTokens, nil
}

// buildCurrEpochValSetMap builds the current epoch's validator set map
// with their original tokens stored in the module state
func (k Keeper) buildCurrEpochValSetMap(ctx context.Context) (map[string]types.ValidatorInfo, error) {
	valMap := make(map[string]types.ValidatorInfo)

	// During genesis, the epoching store may not be initialized yet.
	// In this case, we return an empty map and rely on assumeActiveValidatorIfGenesis
	// to populate validators as needed.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if sdkCtx.BlockHeader().Height == 0 {
		return valMap, nil
	}

	// Get the current epoch's validator set from the epoching keeper
	valSet, err := k.validatorSet.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Convert epoching ValidatorSet to map
	for _, val := range valSet.Validators {
		valAddr := sdk.ValAddress(val.Addr)
		// Get current state of validators from staking keeper
		stkVal, err := k.stkK.GetValidator(ctx, valAddr)
		if err != nil {
			return nil, err
		}

		currentTokens := stkVal.GetTokens()
		valMap[valAddr.String()] = types.ValidatorInfo{
			ValAddress:     valAddr,
			OriginalTokens: val.Tokens,
			CurrentTokens:  currentTokens,
			IsSlashed:      currentTokens.LT(val.Tokens), // consider slashed if current tokens < original tokens
		}
	}

	return valMap, nil
}

// assumeActiveValidatorIfGenesis adds the given validator to the active set if block height is genesis height (0)
// and the validator is not already in the set
func (k Keeper) assumeActiveValidatorIfGenesis(ctx context.Context, valSet map[string]types.ValidatorInfo, valAddr sdk.ValAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if sdkCtx.BlockHeader().Height == 0 {
		// Add validator to active set during genesis
		valSet[valAddr.String()] = types.ValidatorInfo{
			ValAddress:     valAddr,
			IsSlashed:      false,
		}
	}
	return nil
}
