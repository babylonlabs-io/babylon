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

func TestStakingCache_SetAndGetStakedAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr1 := genRandomAddress()
	delAddr2 := genRandomAddress()
	valAddr1 := genRandomValidatorAddress()
	valAddr2 := genRandomValidatorAddress()

	amount1 := math.LegacyNewDec(100)
	amount2 := math.LegacyNewDec(200)
	amount3 := math.LegacyNewDec(300)

	// Test setting and getting values
	cache.SetStakedAmount(delAddr1, valAddr1, amount1)
	result := cache.GetStakedAmount(delAddr1, valAddr1)
	require.True(t, amount1.Equal(result))

	// Test setting multiple validators for same delegator
	cache.SetStakedAmount(delAddr1, valAddr2, amount2)
	result1 := cache.GetStakedAmount(delAddr1, valAddr1)
	result2 := cache.GetStakedAmount(delAddr1, valAddr2)
	require.True(t, amount1.Equal(result1))
	require.True(t, amount2.Equal(result2))

	// Test setting same validator for different delegators
	cache.SetStakedAmount(delAddr2, valAddr1, amount3)
	result1 = cache.GetStakedAmount(delAddr1, valAddr1)
	result2 = cache.GetStakedAmount(delAddr2, valAddr1)
	require.True(t, amount1.Equal(result1))
	require.True(t, amount3.Equal(result2))
}

func TestStakingCache_GetStakedAmount_NotFound(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Test getting non-existent delegator
	result := cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(result))

	// Add a delegator with different validator
	cache.SetStakedAmount(delAddr, genRandomValidatorAddress(), math.LegacyNewDec(100))

	// Test getting non-existent validator for existing delegator
	result = cache.GetStakedAmount(delAddr, valAddr)
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
	result := cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, initialAmount.Equal(result))

	// Update the amount
	cache.SetStakedAmount(delAddr, valAddr, updatedAmount)
	result = cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, updatedAmount.Equal(result))
}

func TestStakingCache_ZeroAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()
	zeroAmount := math.LegacyZeroDec()

	// Set zero amount
	cache.SetStakedAmount(delAddr, valAddr, zeroAmount)
	result := cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, zeroAmount.Equal(result))
}

func TestStakingCache_NegativeAmount(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()
	negativeAmount := math.LegacyNewDec(-50)

	// Set negative amount (should be allowed by cache, validation should be elsewhere)
	cache.SetStakedAmount(delAddr, valAddr, negativeAmount)
	result := cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, negativeAmount.Equal(result))
}

func TestStakingCache_GetStakedAmount_EdgeCases(t *testing.T) {
	cache := NewStakingCache()

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Test with manually setting nil map (edge case)
	delAddrStr := delAddr.String()
	cache.amtByValByDel[delAddrStr] = nil

	result := cache.GetStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(result))
}
