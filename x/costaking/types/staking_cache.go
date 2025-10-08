package types

import (
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

// Clear removes all entries from the cache
func (sc *StakingCache) Clear() {
	sc.amtByValByDel = make(map[string]map[string]math.LegacyDec)
}

// Delete removes one entry from the cache
func (sc *StakingCache) Delete(delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	delAddrStr := delAddr.String()
	_, exists := sc.amtByValByDel[delAddrStr]
	if !exists {
		return
	}

	valAddrStr := valAddr.String()
	delete(sc.amtByValByDel[delAddrStr], valAddrStr)
}
