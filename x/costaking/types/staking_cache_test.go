package types

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestNewStakingCacheIsNotNil(t *testing.T) {
	cache := NewStakingCache()
	require.NotNil(t, cache)
	require.NotNil(t, cache.amtByValByDel)
}

func TestStakingCacheSetAndGetAndDeleteStakedAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr1 := sdk.AccAddress("delAddr1")
	delAddr2 := sdk.AccAddress("delAddr2")
	valAddr1 := sdk.ValAddress("valAddr1")
	valAddr2 := sdk.ValAddress("valAddr2")

	amount1 := math.LegacyNewDec(100)
	amount2 := math.LegacyNewDec(200)
	amount3 := math.LegacyNewDec(300)

	// not found
	result := cache.GetStakedInfo(delAddr1, valAddr1)
	require.True(t, result.Amount.IsZero())

	cache.SetStakedInfo(delAddr1, valAddr1, amount1, amount1)
	cache.SetStakedInfo(delAddr1, valAddr2, amount2, amount2)
	cache.SetStakedInfo(delAddr2, valAddr1, amount3, amount3)

	// Get and delete values, verifying they return correct amounts
	result1 := cache.GetStakedInfo(delAddr1, valAddr1)
	require.True(t, amount1.Equal(result1.Amount))

	result2 := cache.GetStakedInfo(delAddr1, valAddr2)
	require.True(t, amount2.Equal(result2.Amount))

	result3 := cache.GetStakedInfo(delAddr2, valAddr1)
	require.True(t, amount3.Equal(result3.Amount))

	cache.Clear()

	// Verify all values are deleted (should return zero)
	result = cache.GetStakedInfo(delAddr1, valAddr1)
	require.True(t, result.Amount.IsZero())

	result = cache.GetStakedInfo(delAddr1, valAddr2)
	require.True(t, result.Amount.IsZero())

	result = cache.GetStakedInfo(delAddr2, valAddr1)
	require.True(t, result.Amount.IsZero())

	cache.SetStakedInfo(delAddr1, valAddr1, amount1, amount1)
	cache.SetStakedInfo(delAddr1, valAddr1, amount2, amount2) // overwrite

	result2 = cache.GetStakedInfo(delAddr1, valAddr1)
	require.True(t, amount2.Equal(result2.Amount))
}

func TestStakingCacheGetAndDeleteStakedAmountNilMap(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr := sdk.ValAddress("valAddr")

	// Test with manually setting nil map (edge case)
	delAddrStr := delAddr.String()
	cache.amtByValByDel[delAddrStr] = nil

	result := cache.GetStakedInfo(delAddr, valAddr)
	require.True(t, result.Amount.IsZero())
}

func TestStakingCacheGetAndDeleteStakedAmountPreservesOtherValidators(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr1 := sdk.ValAddress("valAddr1")
	valAddr2 := sdk.ValAddress("valAddr2")
	valAddr3 := sdk.ValAddress("valAddr3")

	amount1 := math.LegacyNewDec(100)
	amount2 := math.LegacyNewDec(200)
	amount3 := math.LegacyNewDec(300)

	// Set up multiple validators for the same delegator
	cache.SetStakedInfo(delAddr, valAddr1, amount1, amount1)
	cache.SetStakedInfo(delAddr, valAddr2, amount2, amount2)
	cache.SetStakedInfo(delAddr, valAddr3, amount3, amount3)

	result := cache.GetStakedInfo(delAddr, valAddr2)
	require.True(t, amount2.Equal(result.Amount))
	// check again the value
	result = cache.GetStakedInfo(delAddr, valAddr2)
	require.True(t, amount2.Equal(result.Amount))

	// Verify the delegator's map still exists and contains the correct validators
	delAddrStr := delAddr.String()
	valMap, exists := cache.amtByValByDel[delAddrStr]
	require.True(t, exists)
	require.Equal(t, 3, len(valMap))

	// Verify specific validators exist in the map
	valAddr1Str := valAddr1.String()
	valAddr2Str := valAddr2.String()
	valAddr3Str := valAddr3.String()

	_, val1Exists := valMap[valAddr1Str]
	_, val2Exists := valMap[valAddr2Str]
	_, val3Exists := valMap[valAddr3Str]

	require.True(t, val1Exists)
	require.True(t, val2Exists)
	require.True(t, val3Exists)

	cache.Clear()

	// Verify the delegator's map was cleaned up
	_, exists = cache.amtByValByDel[delAddrStr]
	require.False(t, exists)
}

func TestStakingCacheClear(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr := sdk.ValAddress("valAddr")
	amount3 := math.LegacyNewDec(300)

	cache.SetStakedInfo(delAddr, valAddr, amount3, amount3)

	result := cache.GetStakedInfo(delAddr, valAddr)
	require.True(t, amount3.Equal(result.Amount))

	cache.Clear()

	result = cache.GetStakedInfo(delAddr, valAddr)
	require.True(t, result.Amount.IsZero())
}

func TestStakingCacheDelete(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr := sdk.ValAddress("valAddr")
	amount := math.LegacyNewDec(100)

	cache.SetStakedInfo(delAddr, valAddr, amount, amount)

	result := cache.GetStakedInfo(delAddr, valAddr)
	require.True(t, amount.Equal(result.Amount))
	require.True(t, amount.Equal(result.Shares))

	cache.Delete(delAddr, valAddr)

	result = cache.GetStakedInfo(delAddr, valAddr)
	require.True(t, result.Amount.IsZero())
	require.True(t, result.Shares.IsZero())
}

func TestStakingCacheDeleteNonExistentDelegator(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr := sdk.ValAddress("valAddr")

	cache.Delete(delAddr, valAddr)

	result := cache.GetStakedInfo(delAddr, valAddr)
	require.True(t, result.Amount.IsZero())
	require.True(t, result.Shares.IsZero())
}

func TestStakingCacheDeleteNonExistentValidator(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr1 := sdk.ValAddress("valAddr1")
	valAddr2 := sdk.ValAddress("valAddr2")
	amount := math.LegacyNewDec(100)

	cache.SetStakedInfo(delAddr, valAddr1, amount, amount)

	cache.Delete(delAddr, valAddr2)

	result := cache.GetStakedInfo(delAddr, valAddr1)
	require.True(t, amount.Equal(result.Amount))
	require.True(t, amount.Equal(result.Shares))
}

func TestStakingCacheDeletePreservesOtherValidators(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr1 := sdk.ValAddress("valAddr1")
	valAddr2 := sdk.ValAddress("valAddr2")
	valAddr3 := sdk.ValAddress("valAddr3")

	amount1 := math.LegacyNewDec(100)
	amount2 := math.LegacyNewDec(200)
	amount3 := math.LegacyNewDec(300)

	cache.SetStakedInfo(delAddr, valAddr1, amount1, amount1)
	cache.SetStakedInfo(delAddr, valAddr2, amount2, amount2)
	cache.SetStakedInfo(delAddr, valAddr3, amount3, amount3)

	cache.Delete(delAddr, valAddr2)

	result1 := cache.GetStakedInfo(delAddr, valAddr1)
	require.True(t, amount1.Equal(result1.Amount))
	require.True(t, amount1.Equal(result1.Shares))

	result2 := cache.GetStakedInfo(delAddr, valAddr2)
	require.True(t, result2.Amount.IsZero())
	require.True(t, result2.Shares.IsZero())

	result3 := cache.GetStakedInfo(delAddr, valAddr3)
	require.True(t, amount3.Equal(result3.Amount))
	require.True(t, amount3.Equal(result3.Shares))
}
