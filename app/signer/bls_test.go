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
			loadedPv := TryLoadBlsFromFile(keyFilePath, passwordFilePath)
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
			loadedPv := TryLoadBlsFromFile(keyFilePath, passwordFilePath)
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

		blsSigner, err := LoadOrGenBlsKey(tempDir, true, "", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		keyFile := DefaultBlsKeyFile(tempDir)
		passwordFile := DefaultBlsPasswordFile(tempDir)

		_, err = os.Stat(keyFile)
		assert.NoError(t, err, "BLS key file should exist at: "+keyFile)
		_, err = os.Stat(passwordFile)
		assert.NoError(t, err, "BLS password file should exist at: "+passwordFile)

		loadedSigner, err := LoadOrGenBlsKey(tempDir, true, "", "", "")
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

		blsSigner, err := LoadOrGenBlsKey(tempDir, false, testPassword, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		keyFile := DefaultBlsKeyFile(tempDir)
		passwordFile := DefaultBlsPasswordFile(tempDir)

		_, err = os.Stat(keyFile)
		assert.NoError(t, err, "BLS key file should exist at: "+keyFile)
		_, err = os.Stat(passwordFile)
		assert.NoError(t, err, "BLS password file should exist at: "+passwordFile)

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, testPassword, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey, err := blsSigner.BlsPubKey()
		assert.NoError(t, err)
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())
	})

	t.Run("invalid directory path", func(t *testing.T) {
		blsSigner, err := LoadOrGenBlsKey("/random-non-existent/path/that/should/not/exist", true, "", "", "")
		assert.Error(t, err)
		assert.Nil(t, blsSigner)
	})

	t.Run("custom key path", func(t *testing.T) {
		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

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

		loadedSigner, err := LoadOrGenBlsKey(tempDir, false, password, passwordPath, customFullPath)
		assert.NoError(t, err)
		assert.NotNil(t, loadedSigner)

		origPubKey := pv.Key.PubKey
		loadedPubKey, err := loadedSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, origPubKey.Bytes(), loadedPubKey.Bytes())

		anotherLoadedSigner, err := LoadOrGenBlsKey(tempDir, false, password, passwordPath, customFullPath)
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
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		testPassword := "env-password-123"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		tempDir := t.TempDir()
		nonExistentFile := filepath.Join(tempDir, "non-existent-password.txt")

		password, err := GetBlsPassword(nonExistentFile)
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)
	})

	t.Run("get password from environment variable with empty file path", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		testPassword := "env-password-empty-path"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		password, err := GetBlsPassword("")
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)
	})

	t.Run("get password from file when env var not set", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		t.Setenv(BlsPasswordEnvVar, "")

		tempDir := t.TempDir()
		passwordFile := filepath.Join(tempDir, "password.txt")
		testPassword := "file-password-456"
		err := os.WriteFile(passwordFile, []byte(testPassword), 0600)
		assert.NoError(t, err)

		password, err := GetBlsPassword(passwordFile)
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)
	})

	t.Run("error when neither env var nor file exists", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		t.Setenv(BlsPasswordEnvVar, "")

		tempDir := t.TempDir()
		nonExistentFile := filepath.Join(tempDir, "non-existent-password.txt")

		_, err := GetBlsPassword(nonExistentFile)
		assert.Error(t, err)
	})

	t.Run("error when env var not set and file path is empty", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		t.Setenv(BlsPasswordEnvVar, "")

		_, err := GetBlsPassword("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no password file path provided")
	})
}

func TestLoadBlsWithEnvVar(t *testing.T) {
	t.Run("load bls with environment variable taking precedence over file", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		blsKeyFile := filepath.Join(tempDir, DefaultBlsKeyName)
		blsPasswordFile := filepath.Join(tempDir, DefaultBlsPasswordName)

		envPassword := "env-password-789"
		t.Setenv(BlsPasswordEnvVar, envPassword)

		bls := GenBls(blsKeyFile, blsPasswordFile, envPassword)
		assert.NotNil(t, bls)

		_, err := os.Stat(blsKeyFile)
		assert.NoError(t, err, "BLS key file should exist")
		_, err = os.Stat(blsPasswordFile)
		assert.NoError(t, err, "BLS password file should exist")

		passwordContent, err := os.ReadFile(blsPasswordFile)
		assert.NoError(t, err)
		assert.Equal(t, envPassword, string(passwordContent))

		loadedBls := TryLoadBlsFromFile(blsKeyFile, blsPasswordFile)
		assert.NotNil(t, loadedBls)
		assert.Equal(t, bls.Key.PubKey.Bytes(), loadedBls.Key.PubKey.Bytes())

		t.Setenv(BlsPasswordEnvVar, "")

		fileLoadedBls := TryLoadBlsFromFile(blsKeyFile, blsPasswordFile)
		assert.NotNil(t, fileLoadedBls)
		assert.Equal(t, bls.Key.PubKey.Bytes(), fileLoadedBls.Key.PubKey.Bytes())
	})

	t.Run("save password to file when env var not set", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		t.Setenv(BlsPasswordEnvVar, "")

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

func TestLoadBlsSignerIfExists(t *testing.T) {
	t.Run("load signer with env var but no password file", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		blsKeyFile := filepath.Join(tempDir, DefaultBlsKeyName)
		nonExistentPasswordFile := filepath.Join(tempDir, "non-existent-password.txt")

		testPassword := "env-password-no-file"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		bls := GenBls(blsKeyFile, "", testPassword)
		assert.NotNil(t, bls)

		_, err := os.Stat(blsKeyFile)
		assert.NoError(t, err, "BLS key file should exist")
		_, err = os.Stat(nonExistentPasswordFile)
		assert.Error(t, err, "Password file should not exist")

		blsSigner := LoadBlsSignerIfExists(tempDir, nonExistentPasswordFile, blsKeyFile)
		assert.NotNil(t, blsSigner, "Should load signer with env var but no password file")

		loadedPubKey, err := blsSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, bls.Key.PubKey.Bytes(), loadedPubKey.Bytes())
	})

	t.Run("env var takes precedence over password file", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		blsKeyFile := filepath.Join(tempDir, DefaultBlsKeyName)
		blsPasswordFile := filepath.Join(tempDir, DefaultBlsPasswordName)

		envPassword := "env-password-precedence"
		t.Setenv(BlsPasswordEnvVar, envPassword)

		bls := GenBls(blsKeyFile, blsPasswordFile, envPassword)
		assert.NotNil(t, bls)

		blsSigner := LoadBlsSignerIfExists(tempDir, blsPasswordFile, blsKeyFile)
		assert.NotNil(t, blsSigner, "Should load signer with env var taking precedence")

		loadedPubKey, err := blsSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, bls.Key.PubKey.Bytes(), loadedPubKey.Bytes())

		t.Setenv(BlsPasswordEnvVar, "")

		fileBlsSigner := LoadBlsSignerIfExists(tempDir, blsPasswordFile, blsKeyFile)
		assert.NotNil(t, fileBlsSigner, "Should load signer with file password")

		fileLoadedPubKey, err := fileBlsSigner.BlsPubKey()
		assert.NoError(t, err)
		assert.Equal(t, bls.Key.PubKey.Bytes(), fileLoadedPubKey.Bytes())
	})

	t.Run("return nil when key file doesn't exist", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		testPassword := "env-password-no-key-file"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		nonExistentKeyFile := filepath.Join(tempDir, "non-existent-key.json")

		blsSigner := LoadBlsSignerIfExists(tempDir, "", nonExistentKeyFile)
		assert.Nil(t, blsSigner, "Should return nil when key file doesn't exist")
	})

	t.Run("return nil when neither env var nor password file exists", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		tempDir := t.TempDir()
		defer os.RemoveAll(tempDir)

		t.Setenv(BlsPasswordEnvVar, "")

		blsKeyFile := filepath.Join(tempDir, DefaultBlsKeyName)
		nonExistentPasswordFile := filepath.Join(tempDir, "non-existent-password.txt")

		bls := GenBls(blsKeyFile, "", "test-password")
		assert.NotNil(t, bls)

		blsSigner := LoadBlsSignerIfExists(tempDir, nonExistentPasswordFile, blsKeyFile)
		assert.Nil(t, blsSigner, "Should return nil when neither env var nor password file exists")
	})
}
