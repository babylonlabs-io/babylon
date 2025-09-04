package types

import (
	"crypto/rand"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func genRandomAddress() sdk.AccAddress {
	addr := make([]byte, 20)
	rand.Read(addr)
	return sdk.AccAddress(addr)
}

func genRandomValidatorAddress() sdk.ValAddress {
	addr := make([]byte, 20)
	rand.Read(addr)
	return sdk.ValAddress(addr)
}

func TestNewStakingCache(t *testing.T) {
	cache := NewStakingCache()
	require.NotNil(t, cache)
	require.NotNil(t, cache.amtByValByDel)
}

func TestStakingCache_SetAndGetAndDeleteStakedAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr1 := genRandomAddress()
	delAddr2 := genRandomAddress()
	valAddr1 := genRandomValidatorAddress()
	valAddr2 := genRandomValidatorAddress()

	amount1 := math.LegacyNewDec(100)
	amount2 := math.LegacyNewDec(200)
	amount3 := math.LegacyNewDec(300)

	// Set up cached values
	cache.SetStakedAmount(delAddr1, valAddr1, amount1)
	cache.SetStakedAmount(delAddr1, valAddr2, amount2)
	cache.SetStakedAmount(delAddr2, valAddr1, amount3)

	// Get and delete values, verifying they return correct amounts
	result1 := cache.GetAndDeleteStakedAmount(delAddr1, valAddr1)
	require.True(t, amount1.Equal(result1))

	result2 := cache.GetAndDeleteStakedAmount(delAddr1, valAddr2)
	require.True(t, amount2.Equal(result2))

	result3 := cache.GetAndDeleteStakedAmount(delAddr2, valAddr1)
	require.True(t, amount3.Equal(result3))

	// Verify all values are deleted (should return zero)
	result := cache.GetAndDeleteStakedAmount(delAddr1, valAddr1)
	require.True(t, math.LegacyZeroDec().Equal(result))

	result = cache.GetAndDeleteStakedAmount(delAddr1, valAddr2)
	require.True(t, math.LegacyZeroDec().Equal(result))

	result = cache.GetAndDeleteStakedAmount(delAddr2, valAddr1)
	require.True(t, math.LegacyZeroDec().Equal(result))
}

func TestStakingCache_GetAndDeleteStakedAmount_NotFound(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Test getting non-existent delegator
	result := cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(result))

	// Add a delegator with different validator
	cache.SetStakedAmount(delAddr, genRandomValidatorAddress(), math.LegacyNewDec(100))

	// Test getting non-existent validator for existing delegator
	result = cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(result))
}

func TestStakingCache_UpdateExistingAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	initialAmount := math.LegacyNewDec(100)
	updatedAmount := math.LegacyNewDec(500)

	// Set initial amount
	cache.SetStakedAmount(delAddr, valAddr, initialAmount)

	// Update the amount
	cache.SetStakedAmount(delAddr, valAddr, updatedAmount)

	// Get and delete should return the updated amount
	result := cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, updatedAmount.Equal(result))

	// Second call should return zero since it was deleted
	result = cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(result))
}

func TestStakingCache_ZeroAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()
	zeroAmount := math.LegacyZeroDec()

	// Set zero amount
	cache.SetStakedAmount(delAddr, valAddr, zeroAmount)
	result := cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, zeroAmount.Equal(result))
}

func TestStakingCache_NegativeAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()
	negativeAmount := math.LegacyNewDec(-50)

	// Set negative amount (should be allowed by cache, validation should be elsewhere)
	cache.SetStakedAmount(delAddr, valAddr, negativeAmount)
	result := cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, negativeAmount.Equal(result))
}

func TestStakingCache_GetAndDeleteStakedAmount_EdgeCases(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Test with manually setting nil map (edge case)
	delAddrStr := delAddr.String()
	cache.amtByValByDel[delAddrStr] = nil

	result := cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(result))
}

func TestStakingCache_GetAndDeleteStakedAmount_PreservesOtherValidators(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr1 := genRandomValidatorAddress()
	valAddr2 := genRandomValidatorAddress()
	valAddr3 := genRandomValidatorAddress()

	amount1 := math.LegacyNewDec(100)
	amount2 := math.LegacyNewDec(200)
	amount3 := math.LegacyNewDec(300)

	// Set up multiple validators for the same delegator
	cache.SetStakedAmount(delAddr, valAddr1, amount1)
	cache.SetStakedAmount(delAddr, valAddr2, amount2)
	cache.SetStakedAmount(delAddr, valAddr3, amount3)

	// Get and delete the middle validator
	result := cache.GetAndDeleteStakedAmount(delAddr, valAddr2)
	require.True(t, amount2.Equal(result))

	// Verify the delegator's map still exists and contains the correct validators
	delAddrStr := delAddr.String()
	valMap, exists := cache.amtByValByDel[delAddrStr]
	require.True(t, exists)
	require.Equal(t, 2, len(valMap)) // should have 2 validators left

	// Verify specific validators exist in the map
	valAddr1Str := valAddr1.String()
	valAddr2Str := valAddr2.String()
	valAddr3Str := valAddr3.String()

	_, val1Exists := valMap[valAddr1Str]
	_, val2Exists := valMap[valAddr2Str]
	_, val3Exists := valMap[valAddr3Str]

	require.True(t, val1Exists)
	require.False(t, val2Exists) // should be deleted
	require.True(t, val3Exists)

	// Get and delete the remaining validators
	result1 := cache.GetAndDeleteStakedAmount(delAddr, valAddr1)
	require.True(t, amount1.Equal(result1))

	result3 := cache.GetAndDeleteStakedAmount(delAddr, valAddr3)
	require.True(t, amount3.Equal(result3))

	// Verify the delegator's map was cleaned up
	_, exists = cache.amtByValByDel[delAddrStr]
	require.False(t, exists)
}

func TestStakingCache_GetAndDeleteStakedAmount_AtomicOperation(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()
	amount := math.LegacyNewDec(150)

	// Set up cached value
	cache.SetStakedAmount(delAddr, valAddr, amount)

	// Get and delete in one operation
	result := cache.GetAndDeleteStakedAmount(delAddr, valAddr)

	// Should return the correct amount
	require.True(t, amount.Equal(result))

	// Calling GetAndDelete again should return zero
	secondResult := cache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(secondResult))

	// Verify delegator map was cleaned up
	delAddrStr := delAddr.String()
	_, exists := cache.amtByValByDel[delAddrStr]
	require.False(t, exists)
}