package schnorr_adaptor_signature_test

import (
	"crypto/sha256"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
)

func FuzzAdaptorSignaturePreSignatureConversion(f *testing.F) {
	// random seeds
	f.Add([]byte("hello"))
	f.Add([]byte("1234567890!@#$%^&*()"))
	f.Add([]byte("1234567891!@#$%^&*()"))
	f.Add([]byte("1234567892!@#$%^&*()"))
	f.Add([]byte("1234567893!@#$%^&*()"))

	f.Fuzz(func(t *testing.T, msg []byte) {
		// Generate a random adaptor signature first
		sk, err := btcec.NewPrivateKey()
		require.NoError(t, err)

		// Generate encryption key
		encKey, _, err := asig.GenKeyPair()
		require.NoError(t, err)

		// Sign the fuzz message
		msgHash := sha256.Sum256(msg)
		originalSig, err := asig.EncSign(sk, encKey, msgHash[:])
		require.NoError(t, err)

		// Convert to pre-signature
		preSig := originalSig.ToSpecBytes()

		// Convert back to adaptor signature
		encKeyBytes := encKey.ToBytes()
		convertedSig, err := asig.NewAdaptorSignatureFromSpecFormat(preSig, encKeyBytes)
		require.NoError(t, err)

		// Verify they are equal
		originalSigBytes, err := originalSig.Marshal()
		require.NoError(t, err)
		convertedSigBytes, err := convertedSig.Marshal()
		require.NoError(t, err)
		require.Equal(t, originalSigBytes, convertedSigBytes)
	})
}
