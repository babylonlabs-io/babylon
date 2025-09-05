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
	result := cache.GetStakedAmount(delAddr1, valAddr1)
	require.True(t, result.IsZero())

	cache.SetStakedAmount(delAddr1, valAddr1, amount1)
	cache.SetStakedAmount(delAddr1, valAddr2, amount2)
	cache.SetStakedAmount(delAddr2, valAddr1, amount3)

	// Get and delete values, verifying they return correct amounts
	result1 := cache.GetStakedAmount(delAddr1, valAddr1)
	require.True(t, amount1.Equal(result1))

	result2 := cache.GetStakedAmount(delAddr1, valAddr2)
	require.True(t, amount2.Equal(result2))

	result3 := cache.GetStakedAmount(delAddr2, valAddr1)
	require.True(t, amount3.Equal(result3))

	cache.Clear()

	// Verify all values are deleted (should return zero)
	result = cache.GetStakedAmount(delAddr1, valAddr1)
	require.True(t, result.IsZero())

	result = cache.GetStakedAmount(delAddr1, valAddr2)
	require.True(t, result.IsZero())

	result = cache.GetStakedAmount(delAddr2, valAddr1)
	require.True(t, result.IsZero())

	cache.SetStakedAmount(delAddr1, valAddr1, amount1)
	cache.SetStakedAmount(delAddr1, valAddr1, amount2)

	result2 = cache.GetStakedAmount(delAddr1, valAddr1)
	require.True(t, amount2.Equal(result2))
}

func TestStakingCacheGetAndDeleteStakedAmountNilMap(t *testing.T) {
	cache := NewStakingCache()

	delAddr := sdk.AccAddress("delAddr")
	valAddr := sdk.ValAddress("valAddr")

	// Test with manually setting nil map (edge case)
	delAddrStr := delAddr.String()
	cache.amtByValByDel[delAddrStr] = nil

	result := cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, result.IsZero())
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
	cache.SetStakedAmount(delAddr, valAddr1, amount1)
	cache.SetStakedAmount(delAddr, valAddr2, amount2)
	cache.SetStakedAmount(delAddr, valAddr3, amount3)

	result := cache.GetStakedAmount(delAddr, valAddr2)
	require.True(t, amount2.Equal(result))
	// check again the value
	result = cache.GetStakedAmount(delAddr, valAddr2)
	require.True(t, amount2.Equal(result))

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

	cache.SetStakedAmount(delAddr, valAddr, amount3)

	result := cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, result.Equal(amount3))

	cache.Clear()

	result = cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, result.IsZero())
}
