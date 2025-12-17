package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
// - If difference is negative, ActiveBaby is subtracted
//
// Note: This hook uses a cache to track previous delegation amounts to calculate the delta.
// Defer: Deletes the value from cache after reading it to avoid cases where an (del, val) pair has more than one action
// in the same block as bond, unbond, bond again
func (h HookStaking) AfterDelegationModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	defer h.k.stkCache.Delete(delAddr, valAddr)
	// Check if validator is in the active set
	active, _, err := h.isActiveValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if !active {
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

	infoBefore := h.k.stkCache.GetStakedInfo(delAddr, valAddr)
	delTokenChange := delTokens.Sub(infoBefore.Amount).TruncateInt()

	// Always update ActiveBaby if validator is in active set
	// (removed the IsSlashed check - slashing is now handled in BeforeValidatorSlashed)
	return h.k.costakerModified(ctx, delAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Add(delTokenChange)
	})
}

// BeforeDelegationRemoved This hook is called when an baby delegation removes his entire baby delegation from one validator.
// The AfterDelegationModified hooks is not called in this case as there is no delegation after is modified, so in costaking
// it should remove all tokens that this pair (del, val) had staked. This value can be achieved by caching the tokens
// prior to BeforeDelegationRemoved hook call, which is done by BeforeDelegationSharesModified.
//
// Defer: Deletes the value from cache after reading it to avoid cases where an (del, val) pair has more than one action
// in the same block as bond, unbond, bond again
func (h HookStaking) BeforeDelegationRemoved(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	defer h.k.stkCache.Delete(delAddr, valAddr)

	// Check if validator is in the active set
	active, _, err := h.isActiveValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if !active {
		// Validator not in active set, skip processing
		return nil
	}

	info := h.k.stkCache.GetStakedInfo(delAddr, valAddr)
	delTokenChange := info.Amount.TruncateInt()
	if delTokenChange.IsZero() {
		return nil
	}

	// Always subtract from ActiveBaby if validator is in active set
	// (removed all delta shares logic - slashing is now handled in BeforeValidatorSlashed)
	return h.k.costakerModified(ctx, delAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Sub(delTokenChange)
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
	active, _, err := h.isActiveValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	fmt.Println(">>> BeforeDelegationSharesModified: val removed from valset, active", valAddr.String(), delAddr.String(), active)

	if !active {
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
	fmt.Println(">>> BeforeDelegationSharesModified: delShares:", del.Shares.String())
	fmt.Println(">>> BeforeDelegationSharesModified: delTokens:", delTokens.String())
	fmt.Println("--------------------------------")

	h.k.stkCache.SetStakedInfo(delAddr, valAddr, delTokens, del.Shares)
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

// BeforeValidatorModified implements types.StakingHooks.
func (h HookStaking) BeforeValidatorModified(ctx context.Context, valAddr sdk.ValAddress) error {
	return nil
}

// BeforeValidatorSlashed implements types.StakingHooks.
// It reduces the ActiveBaby amount for all delegators by the slash fraction
func (h HookStaking) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction math.LegacyDec) error {
	// Check if validator is in the active set
	active, _, err := h.isActiveValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if !active {
		// Validator not in active set, no ActiveBaby to reduce
		return nil
	}

	// Get all delegations to this validator
	delegations, err := h.k.stkK.GetValidatorDelegations(ctx, valAddr)
	if err != nil {
		return err
	}

	// For each delegator, reduce their ActiveBaby by the slash fraction
	for _, del := range delegations {
		delAddr := sdk.MustAccAddressFromBech32(del.DelegatorAddress)

		// Get validator to calculate delegation tokens from shares
		val, err := h.k.stkK.GetValidator(ctx, valAddr)
		if err != nil {
			return err
		}

		// Calculate delegation tokens (before slash)
		delTokens := val.TokensFromShares(del.Shares).TruncateInt()

		// Calculate the amount to slash from ActiveBaby
		slashAmount := fraction.MulInt(delTokens).TruncateInt()

		// Reduce ActiveBaby by the slash amount
		if err := h.k.costakerModified(ctx, delAddr, func(rwdTracker *types.CostakerRewardsTracker) {
			rwdTracker.ActiveBaby = rwdTracker.ActiveBaby.Sub(slashAmount)
		}); err != nil {
			h.k.Logger(ctx).Error(
				"failed to reduce ActiveBaby for slashed validator",
				"delegator", delAddr.String(),
				"validator", valAddr.String(),
				"slash_amount", slashAmount.String(),
				"error", err,
			)
			return err
		}
	}

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
func (k Keeper) TokensFromShares(ctx context.Context, valAddr sdk.ValAddress, delShares math.LegacyDec) (math.LegacyDec, error) {
	val, err := k.stkK.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
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

		// There are 2 cases where a validator's tokens and shares
		// may differ from the original tokens and shares
		// stored in the costaking module state at epoch start:
		// 1. The validator has been slashed - this reduces only validator tokens
		// 2. A redelegation has been slashed - this reduces both tokens and shares on the DESTINATION validator. This preserves the token/share ratio

		// To define if a validator has been slashed, conditions are:
		// - If current tokens < original tokens && current shares == original shares
		// - if current tokens < original tokens && current shares < original shares, there're 2 possible cases: (1) redelegation slashed happened (should NOT flag as slashed), (2) both redelegation and validator slashing happened (should flag as slashed)

		// In conclusion, we can determine if a validator has been slashed
		// by checking if the token/share ratio changed

		isSlashed := false
		currentTokens := stkVal.GetTokens()
		currentShares := stkVal.DelegatorShares
		if currentTokens.LT(val.Tokens) && !val.Shares.IsZero() && !currentShares.IsZero() {
			originalRatio := math.LegacyNewDecFromInt(val.Tokens).Quo(val.Shares)
			currentRatio := math.LegacyNewDecFromInt(currentTokens).Quo(currentShares)

			// Use a small tolerance to account for rounding errors in ratio calculations
			// With 18 decimal precision, we tolerate differences of 1 part in 10^12 (1e-12)
			// This handles precision issues from truncation in slashing/redelegation calculations
			// while still detecting actual validator slashing (which typically changes ratio by ~1% or more)
			diff := originalRatio.Sub(currentRatio).Abs()
			tolerance := originalRatio.Mul(math.LegacyNewDecWithPrec(1, 12)) // 1e-12 relative tolerance

			// If ratios differ beyond tolerance, validator has been slashed
			isSlashed = diff.GT(tolerance)
		}

		valMap[valAddr.String()] = types.ValidatorInfo{
			ValAddress:     valAddr,
			OriginalTokens: val.Tokens,
			OriginalShares: val.Shares,
			IsSlashed:      isSlashed,
		}
	}

	return valMap, nil
}

// assumeActiveValidatorIfGenesis adds the given validator to the active set if block height is genesis height (0)
// and the validator is not already in the set
func (k Keeper) assumeActiveValidatorIfGenesis(ctx context.Context, valSet map[string]types.ValidatorInfo, valAddr sdk.ValAddress) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if sdkCtx.BlockHeader().Height == 0 {
		// Add validator to active set during genesis
		valSet[valAddr.String()] = types.ValidatorInfo{
			ValAddress: valAddr,
			IsSlashed:  false,
		}
	}
}

func (h HookStaking) isActiveValidator(ctx context.Context, valAddr sdk.ValAddress) (bool, types.ValidatorInfo, error) {
	// Check if validator is in the active set
	valSet, err := h.k.stkCache.GetActiveValidatorSet(ctx, h.k.buildCurrEpochValSetMap)
	if err != nil {
		return false, types.ValidatorInfo{}, err
	}

	// NOTE: co-staking genesis is called before staking genesis.
	// The active set will be populated during the staking genesis but after calling the hooks, so the active validators map will be empty.
	// Thus, for testing purposes, we assume all validators are active if the set is empty and block height is 0.
	h.k.assumeActiveValidatorIfGenesis(ctx, valSet, valAddr)

	valInfo, ok := valSet[valAddr.String()]
	if !ok {
		// Validator not in active set, skip processing
		return false, types.ValidatorInfo{}, nil
	}
	return true, valInfo, nil
}
