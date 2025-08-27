package keeper_test

import (
	"testing"
	"math/rand"

	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Cached Tip Tests
// =============================================================================

// TestGetMainChainFrom_CachedTip tests that tip is cached and reused
func TestGetMainChainFrom_CachedTip(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	numHeaders := 5
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 1000
	headers[0] = baseHeader

	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 1000 + uint32(i)
		headers[i] = header
	}

	keeper.InsertHeaderInfos(ctx, headers)

	// Initially, cache should have no tip
	cachedTip := keeper.HeaderCache().GetCachedTip()
	require.Nil(t, cachedTip)

	// First call to GetMainChainFromWithCache - should fetch and cache tip
	result1 := keeper.GetMainChainFromWithCache(ctx, 1002)
	require.Equal(t, 3, len(result1))

	// Verify tip is now cached
	cachedTip = keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, cachedTip)
	require.Equal(t, uint32(1004), cachedTip.Height) // Should be highest header

	// Second call - should use cached tip (no store fetch for tip)
	result2 := keeper.GetMainChainFromWithCache(ctx, 1003)
	require.Equal(t, 2, len(result2))

	// Cached tip should be the same object
	cachedTip2 := keeper.HeaderCache().GetCachedTip()
	require.Equal(t, cachedTip.Height, cachedTip2.Height)
	require.True(t, cachedTip.Hash.Eq(cachedTip2.Hash))
}

// TestGetMainChainFrom_TipConsistencyCheck tests tip consistency across block boundaries (realistic scenario)
func TestGetMainChainFrom_TipConsistencyCheck(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate initial headers
	numHeaders := 3
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 2000
	headers[0] = baseHeader

	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 2000 + uint32(i)
		headers[i] = header
	}

	keeper.InsertHeaderInfos(ctx, headers)

	// Block 1: First call - caches tip
	result1 := keeper.GetMainChainFromWithCache(ctx, 2000)
	require.Equal(t, 3, len(result1))

	originalCachedTip := keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, originalCachedTip)
	require.Equal(t, uint32(2002), originalCachedTip.Height)

	// End of Block 1: Reset cache (as done in EndBlock)
	keeper.ResetHeaderCache()

	// Between blocks: Add a new header to change the tip
	newHeader := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[2])
	newHeader.Height = 2003
	keeper.InsertHeaderInfos(ctx, []*types.BTCHeaderInfo{newHeader})

	// Block 2: Cache is empty, should fetch new tip and return updated results
	result2 := keeper.GetMainChainFromWithCache(ctx, 2000)
	require.Equal(t, 4, len(result2)) // Should include new header

	// Cached tip should be updated with new tip
	updatedCachedTip := keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, updatedCachedTip)
	require.Equal(t, uint32(2003), updatedCachedTip.Height) // New tip height

	// Should not be the same as original
	require.NotEqual(t, originalCachedTip.Height, updatedCachedTip.Height)
}

// TestGetMainChainFrom_CachedTipAfterReset tests tip caching after cache reset
func TestGetMainChainFrom_CachedTipAfterReset(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	headers := make([]*types.BTCHeaderInfo, 3)
	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 3000
	headers[0] = baseHeader

	for i := 1; i < 3; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 3000 + uint32(i)
		headers[i] = header
	}

	keeper.InsertHeaderInfos(ctx, headers)

	// First call - caches tip
	result1 := keeper.GetMainChainFromWithCache(ctx, 3000)
	require.Equal(t, 3, len(result1))

	cachedTip := keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, cachedTip)
	require.Equal(t, uint32(3002), cachedTip.Height)

	// Reset cache (simulating EndBlock)
	keeper.ResetHeaderCache()

	// Cached tip should be nil after reset
	cachedTip = keeper.HeaderCache().GetCachedTip()
	require.Nil(t, cachedTip)

	// Next call should fetch and cache tip again
	result2 := keeper.GetMainChainFromWithCache(ctx, 3001)
	require.Equal(t, 2, len(result2))

	// Tip should be cached again
	cachedTip = keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, cachedTip)
	require.Equal(t, uint32(3002), cachedTip.Height)
}

// TestGetMainChainFrom_TipCacheEfficiency tests that cached tip reduces store calls
func TestGetMainChainFrom_TipCacheEfficiency(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	headers := make([]*types.BTCHeaderInfo, 10)
	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 4000
	headers[0] = baseHeader

	for i := 1; i < 10; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 4000 + uint32(i)
		headers[i] = header
	}

	keeper.InsertHeaderInfos(ctx, headers)

	// Multiple calls within the same "block" (before cache reset)
	// All calls should use the cached tip after the first call

	testCases := []struct {
		startHeight uint32
		expectedLen int
	}{
		{4005, 5},  // headers 4005-4009
		{4007, 3},  // headers 4007-4009
		{4003, 7},  // headers 4003-4009
		{4008, 2},  // headers 4008-4009
		{4000, 10}, // headers 4000-4009 (all)
	}

	for i, tc := range testCases {
		result := keeper.GetMainChainFromWithCache(ctx, tc.startHeight)
		require.Equal(t, tc.expectedLen, len(result), "Test case %d failed", i)

		// After first call, tip should be cached
		if i >= 1 {
			cachedTip := keeper.HeaderCache().GetCachedTip()
			require.NotNil(t, cachedTip, "Tip should be cached after first call")
			require.Equal(t, uint32(4009), cachedTip.Height)
		}
	}

	// All subsequent calls should have used the cached tip
	// (We can't easily measure store calls, but we can verify tip consistency)
	finalCachedTip := keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, finalCachedTip)
	require.Equal(t, uint32(4009), finalCachedTip.Height)
}

// TestResetHeaderCache_PreservesCorrectness tests that cache reset doesn't affect correctness
func TestResetHeaderCache_PreservesCorrectness(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	numHeaders := 10
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 3000
	headers[0] = baseHeader

	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 3000 + uint32(i)
		headers[i] = header
	}

	keeper.InsertHeaderInfos(ctx, headers)

	// Get results before cache reset
	resultBefore := keeper.GetMainChainFromWithCache(ctx, 3005)

	// Reset cache
	keeper.ResetHeaderCache()

	// Get results after cache reset - should be identical
	resultAfter := keeper.GetMainChainFromWithCache(ctx, 3005)

	require.Equal(t, len(resultBefore), len(resultAfter))
	for i := range resultBefore {
		require.Equal(t, resultBefore[i].Height, resultAfter[i].Height)
		require.True(t, resultBefore[i].Hash.Eq(resultAfter[i].Hash))
	}
}
