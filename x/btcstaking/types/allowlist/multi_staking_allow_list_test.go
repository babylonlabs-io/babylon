package allowlist_test

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types/allowlist"
)

func TestLoadMultiStakingAllowList(t *testing.T) {
	hashes, err := allowlist.LoadMultiStakingAllowList()
	require.NoError(t, err)
	require.NotNil(t, hashes)
	require.NotEmpty(t, hashes)

	// Verify each hash is valid
	for _, hash := range hashes {
		require.NotNil(t, hash)
		require.Equal(t, chainhash.HashSize, len(hash.CloneBytes()))
	}

	// Verify expected hashes are present (from placeholder)
	expectedHashes := []string{
		"11f29d946c10d7774ce5c1732f51542171c09aedc6dc6f9ec1dcc68118fbe549",
		"ffc5e728e9c6c961f045b60833b6ebe0e22780b33ac359dac99fd0216a785508",
	}

	require.Len(t, hashes, len(expectedHashes))

	for i, expectedHashStr := range expectedHashes {
		expectedHash, err := chainhash.NewHashFromStr(expectedHashStr)
		require.NoError(t, err)
		require.Equal(t, expectedHash, hashes[i])
	}
}

func TestIsMultiStakingAllowListEnabled(t *testing.T) {
	tests := []struct {
		name     string
		height   int64
		expected bool
	}{
		{
			name:     "height below expiration - enabled",
			height:   2,
			expected: true,
		},
		{
			name:     "height at expiration - disabled",
			height:   5,
			expected: false,
		},
		{
			name:     "height above expiration - disabled",
			height:   15,
			expected: false,
		},
		{
			name:     "zero height - enabled",
			height:   0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := allowlist.IsMultiStakingAllowListEnabled(tt.height)
			require.Equal(t, tt.expected, result)
		})
	}
}
