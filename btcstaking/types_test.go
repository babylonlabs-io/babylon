package btcstaking_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/btcstaking"
	"github.com/stretchr/testify/require"
)

func TestIsSlashingRateValid(t *testing.T) {
	testCases := []struct {
		name     string
		rate     sdkmath.LegacyDec
		expected bool
	}{
		{
			name:     "valid rate - 0.5",
			rate:     sdkmath.LegacyNewDecWithPrec(5, 1), // 0.5
			expected: true,
		},
		{
			name:     "valid rate - 0.25",
			rate:     sdkmath.LegacyNewDecWithPrec(25, 2), // 0.25
			expected: true,
		},
		{
			name:     "valid rate - 0.255",
			rate:     sdkmath.LegacyNewDecWithPrec(255, 3), // 0.255
			expected: true,
		},
		{
			name:     "valid rate - 0.2555",
			rate:     sdkmath.LegacyNewDecWithPrec(2555, 4), // 0.2555
			expected: true,
		},
		{
			name:     "valid rate - 0.99",
			rate:     sdkmath.LegacyNewDecWithPrec(99, 2), // 0.99
			expected: true,
		},
		{
			name:     "invalid rate - 0",
			rate:     sdkmath.LegacyZeroDec(),
			expected: false,
		},
		{
			name:     "invalid rate - 1",
			rate:     sdkmath.LegacyOneDec(),
			expected: false,
		},
		{
			name:     "invalid rate - negative",
			rate:     sdkmath.LegacyNewDecWithPrec(-5, 1), // -0.5
			expected: false,
		},
		{
			name:     "invalid rate - too many decimal places",
			rate:     sdkmath.LegacyNewDecWithPrec(25555, 5), // 0.25555
			expected: false,
		},
		{
			name:     "invalid rate - greater than 1",
			rate:     sdkmath.LegacyNewDecWithPrec(15, 1), // 1.5
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := btcstaking.IsSlashingRateValid(tc.rate)
			require.Equal(t, tc.expected, result)
		})
	}
}
