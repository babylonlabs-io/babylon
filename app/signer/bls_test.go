package signer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/test-go/testify/assert"
)

func TestNewBls(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	keyFilePath := DefaultBlsKeyFile(tempDir)
	passwordFilePath := DefaultBlsPasswordFile(tempDir)

	err := EnsureDirs(keyFilePath, passwordFilePath)
	assert.NoError(t, err)

	t.Run("failed when private key is nil", func(t *testing.T) {
		assert.Panics(t, func() {
			NewBls(nil, keyFilePath, passwordFilePath)
		})
	})

	t.Run("save bls key to file without delegator address", func(t *testing.T) {
		pv := NewBls(bls12381.GenPrivKey(), keyFilePath, passwordFilePath)
		assert.NotNil(t, pv)

		password := "password"
		pv.Key.Save(password)

		t.Run("load bls key from file", func(t *testing.T) {
			loadedPv := LoadBls(keyFilePath, passwordFilePath)
			assert.NotNil(t, loadedPv)

			assert.Equal(t, pv.Key.PrivKey, loadedPv.Key.PrivKey)
			assert.Equal(t, pv.Key.PubKey.Bytes(), loadedPv.Key.PubKey.Bytes())
		})
	})

	t.Run("save bls key to file with delegator address", func(t *testing.T) {
		pv := NewBls(bls12381.GenPrivKey(), keyFilePath, passwordFilePath)
		assert.NotNil(t, pv)

		password := "password"
		pv.Key.Save(password)

		t.Run("load bls key from file", func(t *testing.T) {
			loadedPv := LoadBls(keyFilePath, passwordFilePath)
			assert.NotNil(t, loadedPv)

			assert.Equal(t, pv.Key.PrivKey, loadedPv.Key.PrivKey)
			assert.Equal(t, pv.Key.PubKey.Bytes(), loadedPv.Key.PubKey.Bytes())
		})
	})
}

func TestLoadOrGenBlsKey(t *testing.T) {
	t.Run("generate new key without password", func(t *testing.T) {
		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		blsSigner, err := LoadOrGenBlsKey(tempDir, true, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		_, err = os.Stat(DefaultBlsKeyFile(tempDir))
		assert.NoError(t, err, "BLS key file should exist")
		_, err = os.Stat(DefaultBlsPasswordFile(tempDir))
		assert.NoError(t, err, "BLS password file should exist")

		loadedSigner, err := LoadOrGenBlsKey(tempDir, true, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey, err := blsSigner.BlsPubKey()
		assert.NoError(t, err)
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())
	})

	t.Run("generate new key with password", func(t *testing.T) {
		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		testPassword := "testpassword123"

		blsSigner, err := LoadOrGenBlsKey(tempDir, false, testPassword, "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		_, err = os.Stat(DefaultBlsKeyFile(tempDir))
		assert.NoError(t, err, "BLS key file should exist")
		_, err = os.Stat(DefaultBlsPasswordFile(tempDir))
		assert.NoError(t, err, "BLS password file should exist")

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, testPassword, "")
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey, err := blsSigner.BlsPubKey()
		assert.NoError(t, err)
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())
	})

	t.Run("load existing key", func(t *testing.T) {
		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		password := "existingpassword"
		originalSigner, err := CreateBlsSigner(tempDir, password, "")
		assert.NoError(t, err)
		assert.NotNil(t, originalSigner)

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, "different_password", "")
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey, err := originalSigner.BlsPubKey()
		assert.NoError(t, err)
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())
	})

	t.Run("invalid directory path", func(t *testing.T) {
		blsSigner, err := LoadOrGenBlsKey("/random-non-existent/path/that/should/not/exist", true, "", "")
		assert.Error(t, err)
		assert.Nil(t, blsSigner)
	})

	t.Run("custom key path", func(t *testing.T) {
		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		customPath := filepath.Join(tempDir, "custom", "path", "bls_key.json")
		passwordPath := DefaultBlsPasswordFile(tempDir)

		err := os.MkdirAll(filepath.Dir(customPath), 0700)
		assert.NoError(t, err, "Should be able to create custom directory for key")
		err = os.MkdirAll(filepath.Dir(passwordPath), 0700)
		assert.NoError(t, err, "Should be able to create directory for password file")

		password := "testpassword"
		pv := NewBls(bls12381.GenPrivKey(), customPath, passwordPath)
		assert.NotNil(t, pv)
		pv.Key.Save(password)

		_, err = os.Stat(customPath)
		assert.NoError(t, err, "Custom BLS key file should exist")
		_, err = os.Stat(passwordPath)
		assert.NoError(t, err, "BLS password file should exist in default location")

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, password, customPath)
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey := pv.Key.PubKey
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())

		anotherLoadedSigner, err := LoadOrGenBlsKey(tempDir, false, password, customPath)
		assert.NoError(t, err)
		assert.NotNil(t, anotherLoadedSigner)

		anotherLoadedPubKey, err := anotherLoadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), anotherLoadedPubKey.Bytes())
	})
}
