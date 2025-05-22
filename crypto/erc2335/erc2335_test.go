package erc2335

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/test-go/testify/require"
)

const password string = "password"

func TestEncryptBLS(t *testing.T) {
	t.Run("create bls key", func(t *testing.T) {
		blsPrivKey := bls12381.GenPrivKey()
		blsPubKey := blsPrivKey.PubKey().Bytes()

		t.Run("encrypt bls key", func(t *testing.T) {
			encryptedBlsKey, err := Encrypt(blsPrivKey, blsPubKey, password)
			require.NoError(t, err)
			t.Logf("encrypted bls key: %s", encryptedBlsKey)

			t.Run("decrypt bls key", func(t *testing.T) {
				var keystore Erc2335KeyStore
				err = json.Unmarshal(encryptedBlsKey, &keystore)
				require.NoError(t, err)

				decryptedBlsKey, err := Decrypt(keystore, password)
				require.NoError(t, err)
				require.Equal(t, blsPrivKey, bls12381.PrivateKey(decryptedBlsKey))
			})

			t.Run("decrypt bls key with wrong password", func(t *testing.T) {
				var keystore Erc2335KeyStore
				err = json.Unmarshal(encryptedBlsKey, &keystore)
				require.NoError(t, err)
				_, err := Decrypt(keystore, "wrong password")
				require.Error(t, err)
			})
		})

		t.Run("save password and encrypt bls key", func(t *testing.T) {
			encryptedBlsKey, err := Encrypt(blsPrivKey, blsPubKey, password)
			require.NoError(t, err)
			t.Logf("encrypted bls key: %s", encryptedBlsKey)
			err = tempfile.WriteFileAtomic("password.txt", []byte(password), 0600)
			require.NoError(t, err)

			t.Run("load password and decrypt bls key", func(t *testing.T) {
				passwordBytes, err := os.ReadFile("password.txt")
				require.NoError(t, err)
				password := string(passwordBytes)

				var keystore Erc2335KeyStore
				err = json.Unmarshal(encryptedBlsKey, &keystore)
				require.NoError(t, err)

				decryptedBlsKey, err := Decrypt(keystore, password)
				require.NoError(t, err)
				require.Equal(t, blsPrivKey, bls12381.PrivateKey(decryptedBlsKey))
			})

			t.Run("save new password into same file", func(t *testing.T) {
				newPassword := "new password"
				err = tempfile.WriteFileAtomic("password.txt", []byte(newPassword), 0600)
				require.NoError(t, err)
			})

			t.Run("failed when load different password and decrypt bls key", func(t *testing.T) {
				passwordBytes, err := os.ReadFile("password.txt")
				require.NoError(t, err)
				password := string(passwordBytes)

				var keystore Erc2335KeyStore
				err = json.Unmarshal(encryptedBlsKey, &keystore)
				require.NoError(t, err)

				_, err = Decrypt(keystore, password)
				require.Error(t, err)
			})

			t.Run("failed when password file don't exist", func(t *testing.T) {
				_, err := os.ReadFile("nopassword.txt")
				require.Error(t, err)
			})
		})

		t.Run("clean test files", func(t *testing.T) {
			_ = os.RemoveAll("password.txt")
		})
	})
}
