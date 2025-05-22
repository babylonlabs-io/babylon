package schnorr_adaptor_signature_test

import (
	"testing"

	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	"github.com/stretchr/testify/require"
)

func FuzzKeyGen(f *testing.F) {
	// random seeds
	f.Add([]byte("hello"))
	f.Add([]byte("1234567890!@#$%^&*()"))
	f.Add([]byte("1234567891!@#$%^&*()"))
	f.Add([]byte("1234567892!@#$%^&*()"))
	f.Add([]byte("1234567893!@#$%^&*()"))

	f.Fuzz(func(t *testing.T, seed []byte) {
		encKey, decKey, err := asig.GenKeyPair()
		require.NoError(t, err)

		// ensure that decKey.GetEncKey() is same as encKey
		actualEncKey, err := decKey.GetEncKey()
		require.NoError(t, err)
		require.Equal(t, encKey, actualEncKey)

		// ensure that the corresponding btcPK and btcSK
		// constitute a key pair
		btcPK, err := encKey.ToBTCPK()
		require.NoError(t, err)
		btcSK := decKey.ToBTCSK()
		actualBTCPK := btcSK.PubKey()
		require.Equal(t, btcPK, actualBTCPK)

		// ensure that one can convert btcPK and btcSK back to
		// encKey and decKey
		actualEncKey, err = asig.NewEncryptionKeyFromBTCPK(btcPK)
		require.NoError(t, err)
		require.Equal(t, encKey, actualEncKey)
		actualDecKey, err := asig.NewDecryptionKeyFromBTCSK(btcSK)
		require.NoError(t, err)
		require.Equal(t, decKey, actualDecKey)
	})
}

func FuzzKeySerialization(f *testing.F) {
	// random seeds
	f.Add([]byte("hello"))
	f.Add([]byte("1234567890!@#$%^&*()"))
	f.Add([]byte("1234567891!@#$%^&*()"))
	f.Add([]byte("1234567892!@#$%^&*()"))
	f.Add([]byte("1234567893!@#$%^&*()"))

	f.Fuzz(func(t *testing.T, seed []byte) {
		encKey, decKey, err := asig.GenKeyPair()
		require.NoError(t, err)

		// roundtrip of serialising/deserialising encKey
		encKeyBytes := encKey.ToBytes()
		actualEncKey, err := asig.NewEncryptionKeyFromBytes(encKeyBytes)
		require.NoError(t, err)
		require.Equal(t, encKey, actualEncKey)

		// roundtrip of serialising/deserialising decKey
		decKeyBytes := decKey.ToBytes()
		actualDecKey, err := asig.NewDecryptionKeyFromBytes(decKeyBytes)
		require.NoError(t, err)
		require.Equal(t, decKey, actualDecKey)
	})
}
