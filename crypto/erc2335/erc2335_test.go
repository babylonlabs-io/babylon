package erc2335

import (
	"os"
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/test-go/testify/require"
)

func TestEncryptBLS(t *testing.T) {
	// TODO
	t.Run("create bls key", func(t *testing.T) {

		blsPrivKey := bls12381.GenPrivKey()
		blsPubKey := blsPrivKey.PubKey().Bytes()

		t.Run("encrypt bls key", func(t *testing.T) {

			password := CreateRandomPassword()
			t.Logf("password: %s", password)

			encryptedBlsKey, err := Encrypt(blsPrivKey, blsPubKey, password)
			require.NoError(t, err)
			t.Logf("encrypted bls key: %s", encryptedBlsKey)

			t.Run("decrypt bls key", func(t *testing.T) {

				decryptedBlsKey, err := Decrypt(encryptedBlsKey, password)
				require.NoError(t, err)
				require.Equal(t, blsPrivKey, bls12381.PrivateKey(decryptedBlsKey))
			})

			t.Run("decrypt bls key with wrong password", func(t *testing.T) {

				_, err := Decrypt(encryptedBlsKey, "wrong password")
				require.Error(t, err)
			})
		})

		t.Run("save password and encrypt bls key", func(t *testing.T) {

			password := CreateRandomPassword()
			t.Logf("password: %s", password)

			encryptedBlsKey, err := Encrypt(blsPrivKey, blsPubKey, password)
			require.NoError(t, err)
			t.Logf("encrypted bls key: %s", encryptedBlsKey)
			err = SavePasswordToFile(password, "password.txt")
			require.NoError(t, err)

			t.Run("load password and decrypt bls key", func(t *testing.T) {

				password, err := LoadPaswordFromFile("password.txt")
				require.NoError(t, err)
				decryptedBlsKey, err := Decrypt(encryptedBlsKey, password)
				require.NoError(t, err)
				require.Equal(t, blsPrivKey, bls12381.PrivateKey(decryptedBlsKey))
			})

			t.Run("save new password into same file", func(t *testing.T) {

				newPassword := CreateRandomPassword()
				t.Logf("new password: %s", newPassword)
				err = SavePasswordToFile(newPassword, "password.txt")
				require.NoError(t, err)
			})

			t.Run("failed when load different password and decrypt bls key", func(t *testing.T) {

				password, err := LoadPaswordFromFile("password.txt")
				require.NoError(t, err)
				_, err = Decrypt(encryptedBlsKey, password)
				require.Error(t, err)
			})

			t.Run("failed when password file don't exist", func(t *testing.T) {
				_, err := LoadPaswordFromFile("nopassword.txt")
				require.Error(t, err)
			})
		})

		t.Run("clean test files", func(t *testing.T) {
			_ = os.RemoveAll("password.txt")
		})
	})
}
