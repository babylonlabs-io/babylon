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
	
	// First call to GetMainChainFrom - should fetch and cache tip
	result1 := keeper.GetMainChainFrom(ctx, 1002)
	require.Equal(t, 3, len(result1))
	
	// Verify tip is now cached
	cachedTip = keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, cachedTip)
	require.Equal(t, uint32(1004), cachedTip.Height) // Should be highest header
	
	// Second call - should use cached tip (no store fetch for tip)
	result2 := keeper.GetMainChainFrom(ctx, 1003)
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
	result1 := keeper.GetMainChainFrom(ctx, 2000)
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
	result2 := keeper.GetMainChainFrom(ctx, 2000)
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
	result1 := keeper.GetMainChainFrom(ctx, 3000)
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
	result2 := keeper.GetMainChainFrom(ctx, 3001)
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
		{4005, 5}, // headers 4005-4009
		{4007, 3}, // headers 4007-4009  
		{4003, 7}, // headers 4003-4009
		{4008, 2}, // headers 4008-4009
		{4000, 10}, // headers 4000-4009 (all)
	}
	
	for i, tc := range testCases {
		result := keeper.GetMainChainFrom(ctx, tc.startHeight)
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

// =============================================================================
// EndBlock Tests
// =============================================================================

// TestResetHeaderCache_EndBlock tests that header cache is properly reset
func TestResetHeaderCache_EndBlock(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate and insert headers to populate cache
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
	
	// Use GetMainChainFrom to populate cache
	result := keeper.GetMainChainFrom(ctx, 1002)
	require.Equal(t, 3, len(result)) // Should get headers 1002, 1003, 1004
	
	// Verify cache has data
	stats := keeper.HeaderCache().Stats()
	require.Equal(t, int64(3), stats.MissCount) // Initial misses
	require.Equal(t, 3, stats.Size) // 3 cached headers
	
	// Call GetMainChainFrom again - should hit cache
	result2 := keeper.GetMainChainFrom(ctx, 1002)
	require.Equal(t, 3, len(result2))
	
	stats2 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(3), stats2.HitCount) // All should be cache hits
	require.Equal(t, 3, stats2.Size) // Still 3 cached headers
	
	// Reset cache (simulating EndBlock)
	keeper.ResetHeaderCache()
	
	// Verify cache is empty
	stats3 := keeper.HeaderCache().Stats()
	require.Equal(t, 0, stats3.Size) // Cache should be empty
	
	// Call GetMainChainFrom again - should miss cache and repopulate
	result3 := keeper.GetMainChainFrom(ctx, 1002)
	require.Equal(t, 3, len(result3))
	
	stats4 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(6), stats4.MissCount) // 3 more misses (cache was reset)
	require.Equal(t, 3, stats4.Size) // Cache repopulated
	
	// Verify hit count was not reset (statistics are cumulative)
	require.Equal(t, int64(3), stats4.HitCount) // Hit count preserved
}

// TestResetHeaderCache_MemoryBounds tests that cache reset prevents memory growth
func TestResetHeaderCache_MemoryBounds(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate large number of headers
	numHeaders := 50
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
	
	// Populate cache with all headers
	keeper.GetMainChainFrom(ctx, 2000)
	
	initialStats := keeper.HeaderCache().Stats()
	require.Equal(t, numHeaders, initialStats.Size)
	
	// Reset cache multiple times (simulating multiple EndBlocks)
	for i := 0; i < 10; i++ {
		keeper.ResetHeaderCache()
		
		// Verify cache is empty after each reset
		stats := keeper.HeaderCache().Stats()
		require.Equal(t, 0, stats.Size, "Cache should be empty after reset %d", i)
		
		// Repopulate cache with subset of headers
		keeper.GetMainChainFrom(ctx, 2040) // Only last 10 headers
		
		stats = keeper.HeaderCache().Stats()
		require.Equal(t, 10, stats.Size, "Cache should only have 10 headers after repopulation %d", i)
	}
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
	resultBefore := keeper.GetMainChainFrom(ctx, 3005)
	
	// Reset cache
	keeper.ResetHeaderCache()
	
	// Get results after cache reset - should be identical
	resultAfter := keeper.GetMainChainFrom(ctx, 3005)
	
	require.Equal(t, len(resultBefore), len(resultAfter))
	for i := range resultBefore {
		require.Equal(t, resultBefore[i].Height, resultAfter[i].Height)
		require.True(t, resultBefore[i].Hash.Eq(resultAfter[i].Hash))
	}
}

// =============================================================================
// Cache Lifecycle Tests  
// =============================================================================

// TestHeaderCache_FullLifecycle tests the full lifecycle of header cache across blocks
func TestHeaderCache_FullLifecycle(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Test EndBlock functionality directly through keeper
	// (We don't need to test the module wrapper, just the core functionality)
	
	// Generate headers
	numHeaders := 10
	headers := make([]*types.BTCHeaderInfo, numHeaders)
	
	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 5000
	headers[0] = baseHeader
	
	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 5000 + uint32(i)
		headers[i] = header
	}
	
	keeper.InsertHeaderInfos(ctx, headers)
	
	// Simulate Block 1: Multiple operations within same block
	t.Log("=== Block 1: Multiple operations within same block ===")
	
	// Operation 1: GetMainChainFrom - should populate cache
	result1 := keeper.GetMainChainFrom(ctx, 5005)
	require.Equal(t, 5, len(result1))
	
	stats1 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(5), stats1.MissCount)
	require.Equal(t, 5, stats1.Size)
	t.Logf("After op1: Cache size=%d, Misses=%d, Hits=%d", stats1.Size, stats1.MissCount, stats1.HitCount)
	
	// Operation 2: Same range - should hit cache
	result2 := keeper.GetMainChainFrom(ctx, 5005) 
	require.Equal(t, 5, len(result2))
	
	stats2 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(5), stats2.HitCount) // All should be cache hits
	require.Equal(t, 5, stats2.Size)
	t.Logf("After op2: Cache size=%d, Misses=%d, Hits=%d", stats2.Size, stats2.MissCount, stats2.HitCount)
	
	// Operation 3: Overlapping range - should partially hit cache
	result3 := keeper.GetMainChainFrom(ctx, 5003)
	require.Equal(t, 7, len(result3)) // headers 5003-5009
	
	stats3 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(10), stats3.HitCount) // 5 more hits (5005-5009 were cached)
	require.Equal(t, int64(7), stats3.MissCount)  // 2 more misses (5003-5004 were not cached) 
	require.Equal(t, 7, stats3.Size) // Now has 7 headers cached
	t.Logf("After op3: Cache size=%d, Misses=%d, Hits=%d", stats3.Size, stats3.MissCount, stats3.HitCount)
	
	// EndBlock 1: Cache should be reset
	t.Log("=== EndBlock 1: Resetting cache ===")
	keeper.ResetHeaderCache() // Simulate EndBlock cache reset
	
	stats4 := keeper.HeaderCache().Stats()
	require.Equal(t, 0, stats4.Size) // Cache should be empty
	require.Equal(t, int64(10), stats4.HitCount) // Hit count preserved (cumulative)
	require.Equal(t, int64(7), stats4.MissCount) // Miss count preserved (cumulative)
	t.Logf("After EndBlock1: Cache size=%d, Misses=%d, Hits=%d", stats4.Size, stats4.MissCount, stats4.HitCount)
	
	// Simulate Block 2: Operations after cache reset
	t.Log("=== Block 2: Operations after cache reset ===")
	
	// Operation 1: Should repopulate cache (all misses)
	result4 := keeper.GetMainChainFrom(ctx, 5007)
	require.Equal(t, 3, len(result4)) // headers 5007-5009
	
	stats5 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(10), stats5.MissCount) // 3 more misses (cache was empty)
	require.Equal(t, 3, stats5.Size) // New cache with 3 headers
	t.Logf("After Block2-op1: Cache size=%d, Misses=%d, Hits=%d", stats5.Size, stats5.MissCount, stats5.HitCount)
	
	// Operation 2: Should hit cache
	result5 := keeper.GetMainChainFrom(ctx, 5008)
	require.Equal(t, 2, len(result5)) // headers 5008-5009
	
	stats6 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(12), stats6.HitCount) // 2 more hits (5008-5009 were cached)
	require.Equal(t, 3, stats6.Size) // Cache size unchanged
	t.Logf("After Block2-op2: Cache size=%d, Misses=%d, Hits=%d", stats6.Size, stats6.MissCount, stats6.HitCount)
	
	// EndBlock 2: Reset cache again
	t.Log("=== EndBlock 2: Resetting cache again ===")
	keeper.ResetHeaderCache() // Simulate EndBlock cache reset
	
	stats7 := keeper.HeaderCache().Stats()
	require.Equal(t, 0, stats7.Size) // Cache should be empty again
	t.Logf("After EndBlock2: Cache size=%d, Misses=%d, Hits=%d", stats7.Size, stats7.MissCount, stats7.HitCount)
}

// TestHeaderCache_ConcurrentOperationsWithinBlock tests cache behavior with concurrent operations
func TestHeaderCache_ConcurrentOperationsWithinBlock(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	numHeaders := 20
	headers := make([]*types.BTCHeaderInfo, numHeaders)
	
	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 6000
	headers[0] = baseHeader
	
	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 6000 + uint32(i)
		headers[i] = header
	}
	
	keeper.InsertHeaderInfos(ctx, headers)
	
	// Simulate BroadcastBTCTimestamps scenario - multiple consumers requesting overlapping ranges
	t.Log("=== Simulating BroadcastBTCTimestamps with 5 consumers ===")
	
	initialStats := keeper.HeaderCache().Stats()
	require.Equal(t, 0, initialStats.Size)
	
	// Consumer 1: Range 6010-6019 (10 headers)
	consumer1Result := keeper.GetMainChainFrom(ctx, 6010)
	require.Equal(t, 10, len(consumer1Result))
	
	stats1 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(10), stats1.MissCount) // All misses
	require.Equal(t, 10, stats1.Size)
	t.Logf("Consumer 1: Cache size=%d, Misses=%d, Hits=%d", stats1.Size, stats1.MissCount, stats1.HitCount)
	
	// Consumer 2: Same range as Consumer 1 (should be all hits)
	consumer2Result := keeper.GetMainChainFrom(ctx, 6010)
	require.Equal(t, 10, len(consumer2Result))
	
	stats2 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(10), stats2.HitCount) // All hits
	require.Equal(t, 10, stats2.Size)
	t.Logf("Consumer 2: Cache size=%d, Misses=%d, Hits=%d", stats2.Size, stats2.MissCount, stats2.HitCount)
	
	// Consumer 3: Range 6015-6019 (5 headers, subset of previous)
	consumer3Result := keeper.GetMainChainFrom(ctx, 6015)
	require.Equal(t, 5, len(consumer3Result))
	
	stats3 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(15), stats3.HitCount) // 5 more hits
	require.Equal(t, 10, stats3.Size) // Cache size unchanged
	t.Logf("Consumer 3: Cache size=%d, Misses=%d, Hits=%d", stats3.Size, stats3.MissCount, stats3.HitCount)
	
	// Consumer 4: Range 6005-6019 (15 headers, extends the range)
	consumer4Result := keeper.GetMainChainFrom(ctx, 6005)
	require.Equal(t, 15, len(consumer4Result))
	
	stats4 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(25), stats4.HitCount) // 10 more hits (6010-6019 were cached)
	require.Equal(t, int64(15), stats4.MissCount) // 5 more misses (6005-6009 were not cached)
	require.Equal(t, 15, stats4.Size) // Now has 15 headers cached
	t.Logf("Consumer 4: Cache size=%d, Misses=%d, Hits=%d", stats4.Size, stats4.MissCount, stats4.HitCount)
	
	// Consumer 5: Range 6012-6019 (8 headers, all should be cached)
	consumer5Result := keeper.GetMainChainFrom(ctx, 6012)
	require.Equal(t, 8, len(consumer5Result))
	
	stats5 := keeper.HeaderCache().Stats()
	require.Equal(t, int64(33), stats5.HitCount) // 8 more hits
	require.Equal(t, int64(15), stats5.MissCount) // Miss count unchanged
	require.Equal(t, 15, stats5.Size) // Cache size unchanged
	t.Logf("Consumer 5: Cache size=%d, Misses=%d, Hits=%d", stats5.Size, stats5.MissCount, stats5.HitCount)
	
	// Calculate efficiency
	totalOperations := stats5.HitCount + stats5.MissCount
	hitRate := float64(stats5.HitCount) / float64(totalOperations)
	t.Logf("=== Final Stats: Hit Rate = %.2f%% (%d hits / %d total operations) ===", 
		hitRate*100, stats5.HitCount, totalOperations)
	
	// In this scenario, we expect high hit rate due to overlapping ranges
	require.Greater(t, hitRate, 0.6, "Hit rate should be > 60% for overlapping consumer requests")
	
	// EndBlock: Reset cache
	keeper.ResetHeaderCache() // Simulate EndBlock cache reset
	
	finalStats := keeper.HeaderCache().Stats()
	require.Equal(t, 0, finalStats.Size) // Cache should be empty
	t.Logf("After EndBlock: Cache reset, size=%d", finalStats.Size)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkGetMainChainFrom_WithCachedTip benchmarks the performance with cached tip
func BenchmarkGetMainChainFrom_WithCachedTip(b *testing.B) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(b)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	numHeaders := 100
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
	
	// Pre-populate cache and tip
	keeper.GetMainChainFrom(ctx, 1050)
	
	b.ResetTimer()
	
	// Benchmark repeated calls - should use cached tip and headers
	for i := 0; i < b.N; i++ {
		startHeight := uint32(1050 + (i % 10))
		result := keeper.GetMainChainFrom(ctx, startHeight)
		if len(result) == 0 {
			b.Errorf("Expected non-empty result for height %d", startHeight)
		}
	}
}

// BenchmarkGetMainChainFrom_TipFetchComparison compares tip fetching strategies
func BenchmarkGetMainChainFrom_TipFetchComparison(b *testing.B) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(b)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	numHeaders := 50
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
	
	b.Run("WithCachedTip", func(b *testing.B) {
		// Pre-populate cache with tip
		keeper.GetMainChainFrom(ctx, 2025)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := keeper.GetMainChainFrom(ctx, 2025)
			if len(result) != 25 {
				b.Errorf("Expected 25 headers, got %d", len(result))
			}
		}
	})
	
	b.Run("CacheClearedEachTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Clear entire cache before each call to force tip fetch
			keeper.ResetHeaderCache()
			result := keeper.GetMainChainFrom(ctx, 2025)
			if len(result) != 25 {
				b.Errorf("Expected 25 headers, got %d", len(result))
			}
		}
	})
}

// BenchmarkGetMainChainFrom_MultipleConsumersWithCachedTip benchmarks multi-consumer scenario
func BenchmarkGetMainChainFrom_MultipleConsumersWithCachedTip(b *testing.B) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(b)
	r := rand.New(rand.NewSource(10))

	// Generate headers
	numHeaders := 30
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
	
	b.ResetTimer()
	
	// Simulate BroadcastBTCTimestamps - multiple consumers with overlapping ranges
	// After first call, tip should be cached and reused for all subsequent calls
	for i := 0; i < b.N; i++ {
		// Consumer 1: from height 3010
		keeper.GetMainChainFrom(ctx, 3010)
		
		// Consumer 2: from height 3010 (same range - cached tip + cached headers)
		keeper.GetMainChainFrom(ctx, 3010)
		
		// Consumer 3: from height 3015 (subset - cached tip + cached headers)
		keeper.GetMainChainFrom(ctx, 3015)
		
		// Consumer 4: from height 3005 (extends range - cached tip + partial cache)
		keeper.GetMainChainFrom(ctx, 3005)
		
		// Consumer 5: from height 3020 (different range - cached tip + some cache)
		keeper.GetMainChainFrom(ctx, 3020)
		
		// Reset cache after each "block" to simulate real usage
		if i % 100 == 99 { // Reset every 100 iterations to simulate periodic resets
			keeper.ResetHeaderCache()
		}
	}
}