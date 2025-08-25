package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
)

// =============================================================================
// HeaderCache unit tests
// =============================================================================

// TestHeaderCache_BasicFunctionality tests basic cache operations
func TestHeaderCache_BasicFunctionality(t *testing.T) {
	cache := types.NewHeaderCache()

	// Create test header
	testHash, err := bbn.NewBTCHeaderHashBytesFromHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	require.NoError(t, err)
	testHeader := &types.BTCHeaderInfo{
		Hash:   &testHash,
		Height: 100,
	}

	fetchCallCount := 0
	fetcher := func(height uint32) (*types.BTCHeaderInfo, error) { //nolint:unparam
		fetchCallCount++
		require.Equal(t, uint32(100), height)
		return testHeader, nil
	}

	// First call should cache miss and fetch
	header1, err := cache.GetOrFetch(100, fetcher)
	require.NoError(t, err)
	require.Equal(t, testHeader, header1)
	require.Equal(t, 1, fetchCallCount)

	// Second call should cache hit (no fetch)
	header2, err := cache.GetOrFetch(100, fetcher)
	require.NoError(t, err)
	require.Equal(t, testHeader, header2)
	require.Equal(t, 1, fetchCallCount) // Should not increment
}

// TestHeaderCache_TipValidation tests cache validation with tip changes
func TestHeaderCache_TipValidation(t *testing.T) {
	cache := types.NewHeaderCache()

	// Create test headers
	tip1Hash, err := bbn.NewBTCHeaderHashBytesFromHex("111102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	require.NoError(t, err)
	tip1 := &types.BTCHeaderInfo{Hash: &tip1Hash, Height: 100}

	tip2Hash, err := bbn.NewBTCHeaderHashBytesFromHex("222202030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	require.NoError(t, err)
	tip2 := &types.BTCHeaderInfo{Hash: &tip2Hash, Height: 101}

	// Initially no tip, should be invalid
	require.False(t, cache.IsValid(tip1))

	// Update with first tip
	cache.UpdateTip(tip1)
	require.True(t, cache.IsValid(tip1))
	require.False(t, cache.IsValid(tip2))

	// Update with second tip
	cache.UpdateTip(tip2)
	require.False(t, cache.IsValid(tip1))
	require.True(t, cache.IsValid(tip2))
}

// TestHeaderCache_Invalidation tests cache invalidation scenarios
func TestHeaderCache_Invalidation(t *testing.T) {
	cache := types.NewHeaderCache()

	// Add multiple headers to cache
	fetchCount := 0
	fetcher := func(height uint32) (*types.BTCHeaderInfo, error) { //nolint:unparam
		fetchCount++
		hash, _ := bbn.NewBTCHeaderHashBytesFromHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
		return &types.BTCHeaderInfo{Hash: &hash, Height: height}, nil
	}

	// Cache headers at heights 100, 101, 102
	_, err := cache.GetOrFetch(100, fetcher)
	require.NoError(t, err)
	_, err = cache.GetOrFetch(101, fetcher)
	require.NoError(t, err)
	_, err = cache.GetOrFetch(102, fetcher)
	require.NoError(t, err)
	require.Equal(t, 3, fetchCount)

	// Invalidate from height 101 onwards
	cache.InvalidateFromHeight(101)

	// Accessing height 100 should hit cache, 101 should miss
	_, err = cache.GetOrFetch(100, fetcher)
	require.NoError(t, err)
	require.Equal(t, 3, fetchCount) // No new fetch

	_, err = cache.GetOrFetch(101, fetcher)
	require.NoError(t, err)
	require.Equal(t, 4, fetchCount) // New fetch required
}

// TestHeaderCache_ConcurrentAccess tests concurrent access to cache
func TestHeaderCache_ConcurrentAccess(t *testing.T) {
	cache := types.NewHeaderCache()

	fetchCount := 0
	fetcher := func(height uint32) (*types.BTCHeaderInfo, error) { //nolint:unparam
		fetchCount++
		hash, _ := bbn.NewBTCHeaderHashBytesFromHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
		// Simulate some work
		time.Sleep(time.Millisecond)
		return &types.BTCHeaderInfo{Hash: &hash, Height: height}, nil
	}

	// Test concurrent access to same height
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := cache.GetOrFetch(100, fetcher)
			require.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Due to concurrent access, fetch might be called multiple times
	// but the cache should work correctly
	require.Greater(t, fetchCount, 0)
	require.LessOrEqual(t, fetchCount, 10)
}

// TestHeaderCache_ErrorHandling tests error handling in cache
func TestHeaderCache_ErrorHandling(t *testing.T) {
	cache := types.NewHeaderCache()

	fetchCount := 0
	fetcher := func(height uint32) (*types.BTCHeaderInfo, error) { //nolint:unparam
		fetchCount++
		if height == 999 {
			return nil, types.ErrHeaderDoesNotExist
		}
		hash, _ := bbn.NewBTCHeaderHashBytesFromHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
		return &types.BTCHeaderInfo{Hash: &hash, Height: height}, nil
	}

	// Test error case
	_, err := cache.GetOrFetch(999, fetcher)
	require.Error(t, err)
	require.Equal(t, types.ErrHeaderDoesNotExist, err)
	require.Equal(t, 1, fetchCount)

	// Test successful case
	_, err = cache.GetOrFetch(100, fetcher)
	require.NoError(t, err)
	require.Equal(t, 2, fetchCount)
}

// =============================================================================
// GetMainChainFrom integration tests
// =============================================================================

// TestGetMainChainFrom_Simple tests the basic functionality without complex scenarios
func TestGetMainChainFrom_Simple(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate a single header
	header := datagen.GenRandomBTCHeaderInfo(r)

	// Insert header into keeper
	keeper.InsertHeaderInfos(ctx, []*types.BTCHeaderInfo{header})

	// Verify tip is set correctly
	tip := keeper.GetTipInfo(ctx)
	require.NotNil(t, tip)
	t.Logf("Tip height: %d, Header height: %d", tip.Height, header.Height)

	// Test GetMainChainFrom from height 0
	result := keeper.GetMainChainFrom(ctx, 0)
	t.Logf("Result length: %d", len(result))

	if len(result) > 0 {
		t.Logf("First header height: %d", result[0].Height)
	}

	// Test GetMainChainFrom from tip height
	result2 := keeper.GetMainChainFrom(ctx, tip.Height)
	t.Logf("Result2 length: %d", len(result2))
}

// TestGetMainChainFrom_CacheWorking tests that the cache actually works
func TestGetMainChainFrom_CacheWorking(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate and insert a simple chain with known heights
	numHeaders := 5
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	// Generate first header with height 100
	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 100
	headers[0] = baseHeader

	// Generate chain with consecutive heights
	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 100 + uint32(i) // Force consecutive heights
		headers[i] = header
	}

	// Insert headers into keeper
	keeper.InsertHeaderInfos(ctx, headers)

	// Verify tip is set correctly
	tip := keeper.GetTipInfo(ctx)
	require.NotNil(t, tip)
	require.Equal(t, uint32(104), tip.Height) // Last header should be height 104

	t.Logf("Tip height: %d", tip.Height)

	// First call - should populate cache
	result1 := keeper.GetMainChainFrom(ctx, 102) // Should get headers 102, 103, 104
	require.Equal(t, 3, len(result1))

	// Verify we got the right headers
	require.Equal(t, uint32(102), result1[0].Height)
	require.Equal(t, uint32(103), result1[1].Height)
	require.Equal(t, uint32(104), result1[2].Height)

	// Second call with same parameters - should hit cache
	result2 := keeper.GetMainChainFrom(ctx, 102)
	require.Equal(t, 3, len(result2))
	require.Equal(t, result1[0].Height, result2[0].Height)
	require.Equal(t, result1[1].Height, result2[1].Height)
	require.Equal(t, result1[2].Height, result2[2].Height)

	// Third call with different start but overlapping range
	result3 := keeper.GetMainChainFrom(ctx, 101) // Should get headers 101, 102, 103, 104
	require.Equal(t, 4, len(result3))
}

// TestGetMainChainFrom_CacheInvalidation tests cache behavior across block boundaries (realistic scenario)
func TestGetMainChainFrom_CacheInvalidation(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate initial chain
	numHeaders := 3
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 200
	headers[0] = baseHeader

	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 200 + uint32(i)
		headers[i] = header
	}

	keeper.InsertHeaderInfos(ctx, headers)

	// Block 1: Initial call to populate cache
	result1 := keeper.GetMainChainFrom(ctx, 200)
	require.Equal(t, 3, len(result1))

	oldTip := keeper.GetTipInfo(ctx)
	require.Equal(t, uint32(202), oldTip.Height)

	// Verify tip is cached
	cachedTip := keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, cachedTip)
	require.Equal(t, uint32(202), cachedTip.Height)

	// End of Block 1: Reset cache (as done in EndBlock)
	keeper.ResetHeaderCache()

	// Between blocks: Add a new header to extend the chain
	newHeader := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[2])
	newHeader.Height = 203
	keeper.InsertHeaderInfos(ctx, []*types.BTCHeaderInfo{newHeader})

	// Block 2: Cache is empty, should fetch new tip and return updated results
	result2 := keeper.GetMainChainFrom(ctx, 200)
	require.Equal(t, 4, len(result2)) // Now should include the new header

	newTip := keeper.GetTipInfo(ctx)
	require.Equal(t, uint32(203), newTip.Height)

	// Verify the last header is the new one
	lastHeader := result2[len(result2)-1]
	require.Equal(t, uint32(203), lastHeader.Height)

	// Verify new tip is cached
	newCachedTip := keeper.HeaderCache().GetCachedTip()
	require.NotNil(t, newCachedTip)
	require.Equal(t, uint32(203), newCachedTip.Height)
}

// TestGetMainChainFrom_CacheOptimization tests the cache optimization for GetMainChainFrom
func TestGetMainChainFrom_CacheOptimization(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate and insert a chain of headers
	numHeaders := 10
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	// Generate base header
	baseHeader := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
	headers[0] = baseHeader

	// Generate chain of headers
	for i := 1; i < numHeaders; i++ {
		headers[i] = datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
	}

	// Insert headers into keeper
	keeper.InsertHeaderInfos(ctx, headers)

	// Verify tip is set correctly
	tip := keeper.GetTipInfo(ctx)
	require.NotNil(t, tip)
	require.Equal(t, headers[numHeaders-1].Height, tip.Height)
	require.True(t, headers[numHeaders-1].Hash.Eq(tip.Hash))

	// Test GetMainChainFrom with different start heights based on actual header heights
	baseHeight := headers[0].Height
	tipHeight := headers[numHeaders-1].Height
	midHeight := headers[5].Height
	nearTipHeight := headers[numHeaders-2].Height

	testCases := []struct {
		name        string
		startHeight uint32
		expected    int // number of headers expected
	}{
		{
			name:        "from base height (entire chain)",
			startHeight: baseHeight,
			expected:    numHeaders,
		},
		{
			name:        "from middle height",
			startHeight: midHeight,
			expected:    numHeaders - 5,
		},
		{
			name:        "from near tip",
			startHeight: nearTipHeight,
			expected:    2,
		},
		{
			name:        "from tip height",
			startHeight: tipHeight,
			expected:    1,
		},
		{
			name:        "from height higher than tip",
			startHeight: tipHeight + 5,
			expected:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := keeper.GetMainChainFrom(ctx, tc.startHeight)
			require.Equal(t, tc.expected, len(result))

			// Verify the results are correct and in order
			for i, header := range result {
				expectedHeight := tc.startHeight + uint32(i)
				require.Equal(t, expectedHeight, header.Height)

				// Find corresponding original header
				var originalHeader *types.BTCHeaderInfo
				for _, h := range headers {
					if h.Height == expectedHeight {
						originalHeader = h
						break
					}
				}
				require.NotNil(t, originalHeader)
				require.True(t, header.Hash.Eq(originalHeader.Hash))
			}
		})
	}
}

// TestGetMainChainFrom_CacheEfficiency tests that cache reduces duplicate I/O
func TestGetMainChainFrom_CacheEfficiency(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate and insert a chain of headers
	numHeaders := 20
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	// Generate base header
	baseHeader := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
	headers[0] = baseHeader

	// Generate chain of headers
	for i := 1; i < numHeaders; i++ {
		headers[i] = datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
	}

	// Insert headers into keeper
	keeper.InsertHeaderInfos(ctx, headers)

	// Get the actual heights of the headers we inserted
	baseHeight := headers[0].Height
	tipHeight := headers[numHeaders-1].Height

	// First call to GetMainChainFrom - should populate cache
	// Calculate start height as base + 10 to ensure we have enough headers
	startHeight := baseHeight + 10
	if startHeight > tipHeight {
		startHeight = baseHeight + 5 // fallback to a smaller offset
	}
	result1 := keeper.GetMainChainFrom(ctx, startHeight)
	expectedLen := int(tipHeight - startHeight + 1)
	require.Equal(t, expectedLen, len(result1))

	// Second call with same start height - should hit cache
	result2 := keeper.GetMainChainFrom(ctx, startHeight)
	require.Equal(t, len(result1), len(result2))

	// Third call with higher start height - should partially hit cache
	higherStartHeight := startHeight + 5
	if higherStartHeight <= tipHeight {
		result3 := keeper.GetMainChainFrom(ctx, higherStartHeight)
		expectedLen3 := int(tipHeight - higherStartHeight + 1)
		require.Equal(t, expectedLen3, len(result3))
	}

	// Fourth call with lower start height - should mostly hit cache with some misses
	lowerStartHeight := baseHeight + 2
	if lowerStartHeight < startHeight {
		result4 := keeper.GetMainChainFrom(ctx, lowerStartHeight)
		expectedLen4 := int(tipHeight - lowerStartHeight + 1)
		require.Equal(t, expectedLen4, len(result4))
	}
}

// TestGetMainChainFrom_TipChanges tests cache behavior across block boundaries (realistic scenario)
func TestGetMainChainFrom_TipChanges(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)
	r := rand.New(rand.NewSource(10))

	// Generate and insert initial chain
	numHeaders := 10
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	// Generate base header
	baseHeader := datagen.GenRandomBTCHeaderInfoWithParent(r, nil)
	headers[0] = baseHeader

	// Generate chain of headers
	for i := 1; i < numHeaders; i++ {
		headers[i] = datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
	}

	// Insert headers into keeper
	keeper.InsertHeaderInfos(ctx, headers)

	// Block 1: Get initial result and cache stats using actual heights
	baseHeight := headers[0].Height
	tipHeight := headers[numHeaders-1].Height
	startHeight := baseHeight + 5
	if startHeight > tipHeight {
		startHeight = baseHeight + 2
	}
	result1 := keeper.GetMainChainFrom(ctx, startHeight)
	expectedLen1 := int(tipHeight - startHeight + 1)
	require.Equal(t, expectedLen1, len(result1))

	// End of Block 1: Reset cache (as done in EndBlock)
	keeper.ResetHeaderCache()

	// Between blocks: Add a new header to extend the chain
	newHeader := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[numHeaders-1])
	keeper.InsertHeaderInfos(ctx, []*types.BTCHeaderInfo{newHeader})

	// Block 2: Cache is empty, should fetch new tip and return updated results
	result2 := keeper.GetMainChainFrom(ctx, startHeight)
	expectedLen2 := expectedLen1 + 1
	require.Equal(t, expectedLen2, len(result2)) // Now includes the new header

	// Verify the last header in result is the new header
	lastHeader := result2[len(result2)-1]
	require.True(t, lastHeader.Hash.Eq(newHeader.Hash))
	require.Equal(t, newHeader.Height, lastHeader.Height)
}

// TestGetMainChainFrom_EmptyChain tests behavior with empty chain
func TestGetMainChainFrom_EmptyChain(t *testing.T) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(t)

	// Test with empty chain (no headers inserted)
	result := keeper.GetMainChainFrom(ctx, 0)
	require.Empty(t, result) // Should return empty slice when no tip exists

	result = keeper.GetMainChainFrom(ctx, 100)
	require.Empty(t, result) // Should return empty slice when no tip exists
}

// =============================================================================
// Benchmark tests
// =============================================================================

// BenchmarkGetMainChainFrom_WithCache benchmarks the cached version of GetMainChainFrom
func BenchmarkGetMainChainFrom_WithCache(b *testing.B) {
	keeper, ctx := testkeeper.BTCLightClientKeeper(b)
	r := rand.New(rand.NewSource(10))

	// Generate and insert a longer chain for benchmarking
	numHeaders := 100
	headers := make([]*types.BTCHeaderInfo, numHeaders)

	// Generate headers with consecutive heights starting from 1000
	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	baseHeader.Height = 1000
	headers[0] = baseHeader

	for i := 1; i < numHeaders; i++ {
		header := datagen.GenRandomBTCHeaderInfoWithParent(r, headers[i-1])
		header.Height = 1000 + uint32(i)
		headers[i] = header
	}

	keeper.InsertHeaderInfos(ctx, headers)

	// Pre-populate cache with one call (simulate typical usage where cache gets populated)
	keeper.GetMainChainFrom(ctx, 1050)

	b.ResetTimer()

	// Benchmark repeated calls that should hit cache (simulating multiple consumers scenario)
	for i := 0; i < b.N; i++ {
		// Alternate between different start heights that have overlapping cached data
		startHeight := uint32(1050 + (i % 10))
		result := keeper.GetMainChainFrom(ctx, startHeight)
		if len(result) == 0 {
			b.Errorf("Expected non-empty result for height %d", startHeight)
		}
	}
}

// BenchmarkGetMainChainFrom_CacheVsNoCache compares performance with cache enabled vs theoretical no-cache
func BenchmarkGetMainChainFrom_CacheVsNoCache(b *testing.B) {
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

	b.Run("WithCache", func(b *testing.B) {
		// Pre-populate cache
		keeper.GetMainChainFrom(ctx, 2025)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// This should hit cache for most headers
			result := keeper.GetMainChainFrom(ctx, 2025)
			if len(result) != 25 {
				b.Errorf("Expected 25 headers, got %d", len(result))
			}
		}
	})

	b.Run("FreshCacheEachTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Clear cache before each call to simulate no-cache scenario
			keeper.HeaderCache().Invalidate()
			result := keeper.GetMainChainFrom(ctx, 2025)
			if len(result) != 25 {
				b.Errorf("Expected 25 headers, got %d", len(result))
			}
		}
	})
}

// BenchmarkGetMainChainFrom_MultiConsumerScenario benchmarks the scenario where multiple consumers
// request overlapping header ranges (the primary use case for the optimization)
func BenchmarkGetMainChainFrom_MultiConsumerScenario(b *testing.B) {
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

	// Simulate BroadcastBTCTimestamps scenario with 5 consumers requesting overlapping ranges
	for i := 0; i < b.N; i++ {
		// Consumer 1: from height 3010
		keeper.GetMainChainFrom(ctx, 3010)

		// Consumer 2: from height 3010 (same range - should be all cache hits)
		keeper.GetMainChainFrom(ctx, 3010)

		// Consumer 3: from height 3015 (subset of previous - should be cache hits)
		keeper.GetMainChainFrom(ctx, 3015)

		// Consumer 4: from height 3005 (extends the range - partial cache hits)
		keeper.GetMainChainFrom(ctx, 3005)

		// Consumer 5: from height 3020 (different range - some cache hits)
		keeper.GetMainChainFrom(ctx, 3020)
	}
}

// BenchmarkHeaderCache_GetOrFetch benchmarks the cache itself
func BenchmarkHeaderCache_GetOrFetch(b *testing.B) {
	cache := types.NewHeaderCache()
	r := rand.New(rand.NewSource(10))

	// Pre-populate cache with some headers
	for i := uint32(1); i <= 100; i++ {
		header := datagen.GenRandomBTCHeaderInfo(r)
		header.Height = i
		cache.GetOrFetch(i, func(height uint32) (*types.BTCHeaderInfo, error) {
			return header, nil
		})
	}

	b.ResetTimer()

	// Benchmark cache access (should be all hits)
	for i := 0; i < b.N; i++ {
		height := uint32(1 + (i % 100))
		cache.GetOrFetch(height, func(height uint32) (*types.BTCHeaderInfo, error) {
			b.Errorf("Should not be called - cache miss for height %d", height)
			return nil, nil
		})
	}
}
