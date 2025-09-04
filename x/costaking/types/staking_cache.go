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

// GetAndDeleteStakedAmount gets the value in the cache if it is found and deletes it after.
// Note: If a value is not found it returns zero dec.
func (sc *StakingCache) GetAndDeleteStakedAmount(delAddr sdk.AccAddress, valAddr sdk.ValAddress) math.LegacyDec {
	delAddrStr := delAddr.String()
	valAddrStr := valAddr.String()

	if sc.amtByValByDel[delAddrStr] == nil {
		return math.LegacyZeroDec()
	}

	amt, found := sc.amtByValByDel[delAddrStr][valAddrStr]
	if !found {
		return math.LegacyZeroDec()
	}

	// Delete the entry after getting the value
	delete(sc.amtByValByDel[delAddrStr], valAddrStr)

	// Clean up empty delegator map
	if len(sc.amtByValByDel[delAddrStr]) == 0 {
		delete(sc.amtByValByDel, delAddrStr)
	}

	return amt
}
