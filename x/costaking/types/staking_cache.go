package types

import (
	context "context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// StakingCache used to cache the change of baby staking hooks
// BeforeDelegationSharesModified sets the value and
// AfterDelegationModified to calculate the delta change.
type StakingCache struct {
	// amtByValByDel stores the amount it had before the delegation
	// was modified DelAddr => ValAddr => Amt
	amtByValByDel map[string]map[string]math.LegacyDec
	// activeValSet caches the current active validator set from epoching keeper
	activeValSet map[string]struct{}
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

// GetActiveValidatorSet returns the cached active validator set, fetching it if not present
func (sc *StakingCache) GetActiveValidatorSet(ctx context.Context, fetchFn func(ctx context.Context) (map[string]struct{}, error)) (map[string]struct{}, error) {
	if sc.activeValSet != nil {
		return sc.activeValSet, nil
	}

	valSet, err := fetchFn(ctx)
	if err != nil {
		return nil, err
	}

	sc.activeValSet = valSet
	return sc.activeValSet, nil
}

// Clear removes all entries from the cache
func (sc *StakingCache) Clear() {
	sc.amtByValByDel = make(map[string]map[string]math.LegacyDec)
	sc.activeValSet = nil
}
