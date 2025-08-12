package types

import (
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
)

// HeaderCache provides in-memory caching for BTC headers to avoid duplicate DB queries
type HeaderCache struct {
	cache map[string]*btclctypes.BTCHeaderInfo
}

// NewHeaderCache creates a new header cache
func NewHeaderCache() *HeaderCache {
	return &HeaderCache{
		cache: make(map[string]*btclctypes.BTCHeaderInfo),
	}
}

// GetHeaderByHash retrieves a header by hash, using cache first, then falling back to DB
func (hc *HeaderCache) GetHeaderByHash(hash *bbn.BTCHeaderHashBytes, fetchHeaderFn func() (*btclctypes.BTCHeaderInfo, error)) (*btclctypes.BTCHeaderInfo, error) {
	hashStr := hash.String()

	// Check cache first
	if header, found := hc.cache[hashStr]; found {
		return header, nil
	}

	// Cache miss - retrieve from DB
	header, err := fetchHeaderFn()
	if err != nil {
		return nil, err
	}

	// Store in cache for future use
	if header != nil {
		hc.cache[hashStr] = header
	}

	return header, nil
}
