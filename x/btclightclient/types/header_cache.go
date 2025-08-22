package types

import (
	"sync/atomic"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
)

// HeaderCache provides caching for individual BTC headers to eliminate
// duplicate KV store I/O operations across multiple GetMainChainFrom calls
type HeaderCache struct {
	// headers stores cached headers by height
	// Note: HeadersObjectPrefix mapping: Height -> BTCHeaderInfo, key is height as unique identifier
	headers map[uint32]*BTCHeaderInfo

	// cached tip - eliminates need to fetch tip from store repeatedly
	cachedTip *BTCHeaderInfo

	// TODO: remove temporary statistics for benchmarking
	hitCount  int64
	missCount int64
}

// NewHeaderCache creates a new header cache with default configuration
func NewHeaderCache() *HeaderCache {
	return &HeaderCache{
		headers: make(map[uint32]*BTCHeaderInfo),
	}
}

// GetOrFetch retrieves a header from cache or fetches it using the provided function
func (c *HeaderCache) GetOrFetch(height uint32, fetcher func(uint32) (*BTCHeaderInfo, error)) (*BTCHeaderInfo, error) {
	// Try cache first
	if cached, exists := c.headers[height]; exists {
		atomic.AddInt64(&c.hitCount, 1)
		return cached, nil
	}

	// Cache miss or expired - fetch from source
	header, err := fetcher(height)
	if err != nil {
		atomic.AddInt64(&c.missCount, 1)
		return nil, err
	}

	// Store in cache
	if header != nil {
		c.headers[height] = header
	}

	atomic.AddInt64(&c.missCount, 1)
	return header, nil
}

// GetCachedTip returns the cached tip if available, nil otherwise
func (c *HeaderCache) GetCachedTip() *BTCHeaderInfo {
	return c.cachedTip
}

// IsValid checks if the cache is valid for the current tip
func (c *HeaderCache) IsValid(currentTip *BTCHeaderInfo) bool {
	if currentTip == nil || c.cachedTip == nil {
		return false
	}

	return c.cachedTip.Height == currentTip.Height &&
		c.cachedTip.Hash != nil &&
		c.cachedTip.Hash.Eq(currentTip.Hash)
}

// UpdateTip updates the cached tip
func (c *HeaderCache) UpdateTip(tip *BTCHeaderInfo) {
	c.cachedTip = tip
}

// Invalidate clears all cached headers and tip
func (c *HeaderCache) Invalidate() {
	c.headers = make(map[uint32]*BTCHeaderInfo)
	c.cachedTip = nil
}

// InvalidateFromHeight removes cached headers at or above the given height
func (c *HeaderCache) InvalidateFromHeight(height uint32) {
	for h := range c.headers {
		if h >= height {
			delete(c.headers, h)
		}
	}
}

// TODO: remove temporary statistics for benchmarking
// Stats returns cache statistics
func (c *HeaderCache) Stats() CacheStats {
	var tipHeight uint32
	var tipHash *bbn.BTCHeaderHashBytes

	if c.cachedTip != nil {
		tipHeight = c.cachedTip.Height
		tipHash = c.cachedTip.Hash
	}

	return CacheStats{
		Size:      len(c.headers),
		HitCount:  atomic.LoadInt64(&c.hitCount),
		MissCount: atomic.LoadInt64(&c.missCount),
		TipHeight: tipHeight,
		TipHash:   tipHash,
	}
}

// CacheStats provides cache metrics
type CacheStats struct {
	Size      int
	HitCount  int64
	MissCount int64
	TipHeight uint32
	TipHash   *bbn.BTCHeaderHashBytes
}

// HitRate returns the cache hit rate
func (stats CacheStats) HitRate() float64 {
	total := stats.HitCount + stats.MissCount
	if total == 0 {
		return 0
	}
	return float64(stats.HitCount) / float64(total)
}
