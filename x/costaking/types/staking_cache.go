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
	amtByValByDel map[string]map[string]StakeInfo
	// activeValSet caches the current active validator set map
	// ValAddr => Tokens
	activeValSet map[string]ValidatorInfo
}

type StakeInfo struct {
	Amount math.LegacyDec
	Shares math.LegacyDec
}

type ValidatorInfo struct {
	ValAddress              sdk.ValAddress
	OriginalTokens          math.Int
	OriginalShares          math.LegacyDec
	CurrentTokens           math.Int
	IsSlashed               bool
	DeltaSharesPerDelegator map[string][]math.LegacyDec // DelAddrStr => []DeltaShares
}

var zeroStakeInfo = StakeInfo{
	Amount: math.LegacyZeroDec(),
	Shares: math.LegacyZeroDec(),
}

func NewStakingCache() *StakingCache {
	return &StakingCache{
		amtByValByDel: make(map[string]map[string]StakeInfo),
	}
}

func (sc *StakingCache) SetStakedInfo(delAddr sdk.AccAddress, valAddr sdk.ValAddress, amtStaked math.LegacyDec, delShares math.LegacyDec) {
	delAddrStr := delAddr.String()
	valAddrStr := valAddr.String()

	if sc.amtByValByDel[delAddrStr] == nil {
		sc.amtByValByDel[delAddrStr] = make(map[string]StakeInfo)
	}
	sc.amtByValByDel[delAddrStr][valAddrStr] = StakeInfo{
		Amount: amtStaked,
		Shares: delShares,
	}
}

// GetStakedInfo gets the value in the cache if it is found.
// Note: If a value is not found it returns zero dec.
func (sc *StakingCache) GetStakedInfo(delAddr sdk.AccAddress, valAddr sdk.ValAddress) StakeInfo {
	delAddrStr := delAddr.String()
	valAddrStr := valAddr.String()

	if sc.amtByValByDel[delAddrStr] == nil {
		return zeroStakeInfo
	}

	info, found := sc.amtByValByDel[delAddrStr][valAddrStr]
	if !found {
		return zeroStakeInfo
	}

	return info
}

// GetActiveValidatorSet returns the cached active validator set, fetching it if not present
func (sc *StakingCache) GetActiveValidatorSet(ctx context.Context, fetchFn func(ctx context.Context) (map[string]ValidatorInfo, error)) (map[string]ValidatorInfo, error) {
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

func (sc *StakingCache) AddDeltaShares(valAddr sdk.ValAddress, delAddr sdk.AccAddress, deltaShares math.LegacyDec) {
	valInfo, ok := sc.activeValSet[valAddr.String()]
	if !ok {
		return
	}

	if valInfo.DeltaSharesPerDelegator == nil {
		valInfo.DeltaSharesPerDelegator = make(map[string][]math.LegacyDec)
	}

	delAddrStr := delAddr.String()
	valInfo.DeltaSharesPerDelegator[delAddrStr] = append(valInfo.DeltaSharesPerDelegator[delAddrStr], deltaShares)
	sc.activeValSet[valAddr.String()] = valInfo
}

func (sc *StakingCache) GetDeltaShares(valAddr sdk.ValAddress, delAddr sdk.AccAddress) []math.LegacyDec {
	valInfo, ok := sc.activeValSet[valAddr.String()]
	if !ok {
		return nil
	}

	if valInfo.DeltaSharesPerDelegator == nil {
		return nil
	}

	delAddrStr := delAddr.String()
	return valInfo.DeltaSharesPerDelegator[delAddrStr]
}

// Clear removes all entries from the cache
func (sc *StakingCache) Clear() {
	sc.amtByValByDel = make(map[string]map[string]StakeInfo)
	sc.activeValSet = nil
}
