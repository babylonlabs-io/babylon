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

		blsSigner, err := LoadOrGenBlsKey(tempDir, true, "", defaultBlsKeyFilePath)
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		_, err = os.Stat(DefaultBlsKeyFile(tempDir))
		assert.NoError(t, err, "BLS key file should exist")
		_, err = os.Stat(DefaultBlsPasswordFile(tempDir))
		assert.NoError(t, err, "BLS password file should exist")

		loadedSigner, err := LoadOrGenBlsKey(tempDir, true, "", defaultBlsKeyFilePath)
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

		blsSigner, err := LoadOrGenBlsKey(tempDir, false, testPassword, defaultBlsKeyFilePath)
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		_, err = os.Stat(DefaultBlsKeyFile(tempDir))
		assert.NoError(t, err, "BLS key file should exist")
		_, err = os.Stat(DefaultBlsPasswordFile(tempDir))
		assert.NoError(t, err, "BLS password file should exist")

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, testPassword, defaultBlsKeyFilePath)
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
		originalSigner, err := CreateBlsSigner(tempDir, password, defaultBlsKeyFilePath)
		assert.NoError(t, err)
		assert.NotNil(t, originalSigner)

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, "different_password", defaultBlsKeyFilePath)
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey, err := originalSigner.BlsPubKey()
		assert.NoError(t, err)
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())
	})

	t.Run("invalid directory path", func(t *testing.T) {
		blsSigner, err := LoadOrGenBlsKey("/random-non-existent/path/that/should/not/exist", true, "", defaultBlsKeyFilePath)
		assert.Error(t, err)
		assert.Nil(t, blsSigner)
	})

	t.Run("custom key path", func(t *testing.T) {
		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		// Create a full path for the custom key location
		customFullPath := filepath.Join(tempDir, "custom", "path", "bls_key.json")
		passwordPath := DefaultBlsPasswordFile(tempDir)

		err := os.MkdirAll(filepath.Dir(customFullPath), 0700)
		assert.NoError(t, err, "Should be able to create custom directory for key")
		err = os.MkdirAll(filepath.Dir(passwordPath), 0700)
		assert.NoError(t, err, "Should be able to create directory for password file")

		password := "testpassword"
		pv := NewBls(bls12381.GenPrivKey(), customFullPath, passwordPath)
		assert.NotNil(t, pv)
		pv.Key.Save(password)

		_, err = os.Stat(customFullPath)
		assert.NoError(t, err, "Custom BLS key file should exist")
		_, err = os.Stat(passwordPath)
		assert.NoError(t, err, "BLS password file should exist in default location")

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, password, customFullPath)
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey := pv.Key.PubKey
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())

		anotherLoadedSigner, err := LoadOrGenBlsKey(tempDir, false, password, customFullPath)
		assert.NoError(t, err)
		assert.NotNil(t, anotherLoadedSigner)

		anotherLoadedPubKey, err := anotherLoadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), anotherLoadedPubKey.Bytes())
	})
}

func TestGetBlsPassword(t *testing.T) {
	t.Run("get password from environment variable", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer os.Setenv(BlsPasswordEnvVar, originalValue)

		testPassword := "env-password-123"
		err := os.Setenv(BlsPasswordEnvVar, testPassword)
		assert.NoError(t, err)

		tempDir := t.TempDir()
		nonExistentFile := filepath.Join(tempDir, "non-existent-password.txt")

		password, fromEnv, err := GetBlsPassword(nonExistentFile)
		assert.NoError(t, err)
		assert.True(t, fromEnv)
		assert.Equal(t, testPassword, password)
	})

	t.Run("get password from file when env var not set", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer os.Setenv(BlsPasswordEnvVar, originalValue)

		err := os.Setenv(BlsPasswordEnvVar, "")
		assert.NoError(t, err)

		tempDir := t.TempDir()
		passwordFile := filepath.Join(tempDir, "password.txt")
		testPassword := "file-password-456"
		err = os.WriteFile(passwordFile, []byte(testPassword), 0600)
		assert.NoError(t, err)

		password, fromEnv, err := GetBlsPassword(passwordFile)
		assert.NoError(t, err)
		assert.False(t, fromEnv)
		assert.Equal(t, testPassword, password)
	})

	t.Run("error when neither env var nor file exists", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer os.Setenv(BlsPasswordEnvVar, originalValue)

		err := os.Setenv(BlsPasswordEnvVar, "")
		assert.NoError(t, err)

		tempDir := t.TempDir()
		nonExistentFile := filepath.Join(tempDir, "non-existent-password.txt")

		_, _, err = GetBlsPassword(nonExistentFile)
		assert.Error(t, err)
	})
}

func TestLoadBlsWithEnvVar(t *testing.T) {
	t.Run("load bls with environment variable", func(t *testing.T) {
		// Save original env var value to restore later
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer os.Setenv(BlsPasswordEnvVar, originalValue)

		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		blsKeyFile := filepath.Join(tempDir, DefaultBlsKeyName)
		blsPasswordFile := filepath.Join(tempDir, DefaultBlsPasswordName)
		filePassword := "file-password-789"

		bls := GenBls(blsKeyFile, blsPasswordFile, filePassword)
		assert.NotNil(t, bls)

		_, err := os.Stat(blsKeyFile)
		assert.NoError(t, err, "BLS key file should exist")
		_, err = os.Stat(blsPasswordFile)
		assert.NoError(t, err, "BLS password file should exist")

		envPassword := "env-password-789"
		err = os.Setenv(BlsPasswordEnvVar, envPassword)
		assert.NoError(t, err)

		originalPasswordContent, err := os.ReadFile(blsPasswordFile)
		assert.NoError(t, err)
		assert.Equal(t, filePassword, string(originalPasswordContent))

		pv := NewBls(bls.Key.PrivKey, blsKeyFile, blsPasswordFile)
		pv.Key.Save(envPassword)

		currentPasswordContent, err := os.ReadFile(blsPasswordFile)
		assert.NoError(t, err)
		assert.Equal(t, filePassword, string(currentPasswordContent))
		assert.Equal(t, string(originalPasswordContent), string(currentPasswordContent))

		loadedBls := LoadBls(blsKeyFile, blsPasswordFile)
		assert.NotNil(t, loadedBls)

		assert.Equal(t, bls.Key.PubKey.Bytes(), loadedBls.Key.PubKey.Bytes())
	})

	t.Run("save password to file when env var not set", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer os.Setenv(BlsPasswordEnvVar, originalValue)

		err := os.Setenv(BlsPasswordEnvVar, "")
		assert.NoError(t, err)

		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		blsKeyFile := filepath.Join(tempDir, DefaultBlsKeyName)
		blsPasswordFile := filepath.Join(tempDir, DefaultBlsPasswordName)
		filePassword := "new-file-password"

		bls := GenBls(blsKeyFile, blsPasswordFile, filePassword)
		assert.NotNil(t, bls)

		passwordContent, err := os.ReadFile(blsPasswordFile)
		assert.NoError(t, err)
		assert.Equal(t, filePassword, string(passwordContent))
	})
}
