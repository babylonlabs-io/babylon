package erc2335

import (
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/test-go/testify/require"
)

func TestEncryptBLS(t *testing.T) {
	// TODO
	t.Run("create bls key", func(t *testing.T) {
		// TODO
		blsPrivKey := bls12381.GenPrivKey()
		blsPubKey := blsPrivKey.PubKey().Bytes()
		password := "password"

		t.Run("encrypt bls key", func(t *testing.T) {
			// TODO
			encryptedBlsKey, err := EncryptBLS(blsPrivKey, blsPubKey, password)
			require.NoError(t, err)
			t.Logf("encrypted bls key: %s", encryptedBlsKey)

			t.Run("decrypt bls key", func(t *testing.T) {
				// TODO
				decryptedBlsKey, err := DecryptBLS(encryptedBlsKey, password)
				require.NoError(t, err)
				require.Equal(t, blsPrivKey, bls12381.PrivateKey(decryptedBlsKey))
			})

			t.Run("decrypt bls key with wrong password", func(t *testing.T) {
				// TODO
				_, err := DecryptBLS(encryptedBlsKey, "wrong password")
				require.Error(t, err)
			})
		})
	})
}
