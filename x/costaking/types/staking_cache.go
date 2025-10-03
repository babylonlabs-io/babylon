package types

import (
	"context"

	"cosmossdk.io/math"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// StakingCache used to cache the change of baby staking hooks
// BeforeDelegationSharesModified sets the value and
// AfterDelegationModified to calculate the delta change.
type StakingCache struct {
	// amtByValByDel stores the amount it had before the delegation
	// was modified DelAddr => ValAddr => Amt
	amtByValByDel map[string]map[string]math.LegacyDec
	// validatorSet caches the current validator set from epoching keeper
	validatorSet *epochingtypes.ValidatorSet
}

func NewStakingCache() *StakingCache {
	return &StakingCache{
		amtByValByDel: make(map[string]map[string]math.LegacyDec),
	}
}

func (sc *StakingCache) SetStakedAmount(delAddr sdk.AccAddress, valAddr sdk.ValAddress, amtStaked math.LegacyDec) {
	delAddrStr := delAddr.String()
	valAddrStr := valAddr.String()

	if sc.amtByValByDel[delAddrStr] == nil {
		sc.amtByValByDel[delAddrStr] = make(map[string]math.LegacyDec)
	}
	sc.amtByValByDel[delAddrStr][valAddrStr] = amtStaked
}

// GetStakedAmount gets the value in the cache if it is found.
// Note: If a value is not found it returns zero dec.
func (sc *StakingCache) GetStakedAmount(delAddr sdk.AccAddress, valAddr sdk.ValAddress) math.LegacyDec {
	delAddrStr := delAddr.String()
	valAddrStr := valAddr.String()

	if sc.amtByValByDel[delAddrStr] == nil {
		return math.LegacyZeroDec()
	}

	amt, found := sc.amtByValByDel[delAddrStr][valAddrStr]
	if !found {
		return math.LegacyZeroDec()
	}

	return amt
}

// GetValidatorSet returns the cached validator set, fetching it if not present
func (sc *StakingCache) GetValidatorSet(ctx context.Context, epochingK EpochingKeeper) epochingtypes.ValidatorSet {
	if sc.validatorSet == nil {
		valSet := epochingK.GetCurrentValidatorSet(ctx)
		sc.validatorSet = &valSet
	}
	return *sc.validatorSet
}

// Clear removes all entries from the cache
func (sc *StakingCache) Clear() {
	sc.amtByValByDel = make(map[string]map[string]math.LegacyDec)
	sc.validatorSet = nil
}
