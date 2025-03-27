package common_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/common"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

func FuzzScalarBaseMultWithBlinding(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(2))
	f.Add(uint64(123456789))
	f.Add(uint64(999999999))
	f.Add(uint64(4294967295))           // Max uint32
	f.Add(uint64(4294967296))           // Max uint32 + 1
	f.Add(uint64(18446744073709551615)) // Max uint64

	f.Fuzz(func(t *testing.T, n uint64) {
		// Create scalar from input
		k := new(btcec.ModNScalar)
		k.SetInt(uint32(n))

		// Get blinded result
		blindedPoint, err := common.ScalarBaseMultWithBlinding(k)
		require.NoError(t, err)

		// Get non-blinded result
		var nonBlindedPoint btcec.JacobianPoint
		btcec.ScalarBaseMultNonConst(k, &nonBlindedPoint)

		// Convert both to affine for comparison
		blindedPoint.ToAffine()
		nonBlindedPoint.ToAffine()

		// Results should be equal
		require.Equal(t, blindedPoint.X, nonBlindedPoint.X)
		require.Equal(t, blindedPoint.Y, nonBlindedPoint.Y)
		require.Equal(t, blindedPoint.Z, nonBlindedPoint.Z)
	})
}
