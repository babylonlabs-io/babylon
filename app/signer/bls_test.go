package signer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/test-go/testify/assert"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
)

// Global command instance for tests that need it
var testCmd = &cobra.Command{}

func TestNewBls(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	keyFilePath := DefaultBlsKeyFile(tempDir)
	passwordFilePath := DefaultBlsPasswordFile(tempDir)

	// This initial EnsureDirs call doesn't help the parallel tests since they create their own temp dirs
	err := EnsureDirs(keyFilePath, passwordFilePath)
	assert.NoError(t, err)

	t.Run("failed when private key is nil", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			NewBls(nil, keyFilePath, passwordFilePath)
		})
	})

	t.Run("save bls key to file", func(t *testing.T) {
		t.Parallel()
		testTempDir := t.TempDir()
		testKeyFilePath := DefaultBlsKeyFile(testTempDir)
		testPasswordFilePath := DefaultBlsPasswordFile(testTempDir)

		err := EnsureDirs(testKeyFilePath, testPasswordFilePath)
		assert.NoError(t, err)

		pv := NewBls(bls12381.GenPrivKey(), testKeyFilePath, testPasswordFilePath)
		assert.NotNil(t, pv)

		password := "password"
		pv.Key.Save(password)

		t.Run("load bls key from file", func(t *testing.T) {
			t.Parallel()
			loadedPv, _, err := TryLoadBlsFromFile(testKeyFilePath, testPasswordFilePath)
			assert.NoError(t, err)
			assert.NotNil(t, loadedPv)

			assert.Equal(t, pv.Key.PrivKey, loadedPv.Key.PrivKey)
			assert.Equal(t, pv.Key.PubKey.Bytes(), loadedPv.Key.PubKey.Bytes())
		})

		t.Run("check bls_key.json and bls_password.txt permissions", func(t *testing.T) {
			t.Parallel()
			// Check permissions
			fileInfo, err := os.Stat(testKeyFilePath)
			assert.NoError(t, err)
			assert.Equal(t, fileInfo.Mode().Perm(), os.FileMode(0400))

			fileInfo, err = os.Stat(testPasswordFilePath)
			assert.NoError(t, err)
			assert.Equal(t, fileInfo.Mode().Perm(), os.FileMode(0400))
		})
	})
}

func TestGetBlsPassword(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("get password from environment variable", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		testPassword := "env-password-123"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		// Verify the env var is set correctly
		envVal, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.True(t, exists)
		assert.Equal(t, testPassword, envVal)

		password, err := GetBlsPassword("")
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)

		tempDir := t.TempDir()
		nonExistentFile := filepath.Join(tempDir, "non-existent-password.txt")
		password, err = GetBlsPassword(nonExistentFile)
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)

		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("get password from file when env var not set", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for this test")

		tempDir := t.TempDir()
		passwordFile := filepath.Join(tempDir, "password.txt")
		testPassword := "file-password-456"
		err := os.WriteFile(passwordFile, []byte(testPassword), 0600)
		assert.NoError(t, err)

		_, err = os.Stat(passwordFile)
		assert.NoError(t, err, "Password file should exist")

		password, err := GetBlsPassword(passwordFile)
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)
	})

	t.Run("error when no password sources available", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for this test")

		tempDir := t.TempDir()
		nonExistentFile := filepath.Join(tempDir, "non-existent-password.txt")

		_, err := GetBlsPassword(nonExistentFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BLS password file does not exist")

		_, err = GetBlsPassword("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BLS password not found in environment variable and no password file path provided")
	})
}

func TestGetBlsKeyFileIfExist(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("error with key does not exists", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		_, exist := GetBlsKeyFileIfExist(tempDir, "")
		assert.False(t, exist)
	})
}

func TestLoadBlsWithEnvVar(t *testing.T) {
	t.Run("load bls with environment variable", func(t *testing.T) {
		originalValue := os.Getenv(BlsPasswordEnvVar)
		defer t.Setenv(BlsPasswordEnvVar, originalValue)

		tempDir := t.TempDir()

		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		blsKeyFile := filepath.Join(configDir, DefaultBlsKeyName)

		testPassword := "test-password-123"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		privKey := bls12381.GenPrivKey()
		blsSigner := NewBls(privKey, blsKeyFile, "")
		blsSigner.Key.Save(testPassword)

		_, err = os.Stat(blsKeyFile)
		assert.NoError(t, err, "BLS key file should exist")

		loadedBls, found, err := TryLoadBlsFromFile(blsKeyFile, "")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.NotNil(t, loadedBls)

		assert.Equal(t, privKey.PubKey().Bytes(), loadedBls.Key.PubKey.Bytes())
	})
}

func TestLoadBlsSignerIfExists(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("load with environment variable", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		blsKeyFile := filepath.Join(configDir, DefaultBlsKeyName)
		testPassword := "env-password-test"

		t.Setenv(BlsPasswordEnvVar, testPassword)

		privKey := bls12381.GenPrivKey()
		bls := NewBls(privKey, blsKeyFile, "")
		bls.Key.Save(testPassword)

		blsSigner, err := LoadBlsSignerIfExists(tempDir, false, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner, "Should load with env var password")

		if blsSigner != nil {
			loadedPubKey, err := blsSigner.BlsPubKey()
			assert.NoError(t, err)
			assert.Equal(t, privKey.PubKey().Bytes(), loadedPubKey.Bytes())
		}
		// Clean up env var after this test
		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("load with password file", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		passwordFile := filepath.Join(tempDir, "password.txt")
		testPassword := "file-password-test"
		err = os.WriteFile(passwordFile, []byte(testPassword), 0600)
		assert.NoError(t, err)

		blsKeyFile := filepath.Join(configDir, DefaultBlsKeyName)
		privKey := bls12381.GenPrivKey()
		bls := NewBls(privKey, blsKeyFile, "")
		bls.Key.Save(testPassword)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should not be set for password file test")

		blsSigner, err := LoadBlsSignerIfExists(tempDir, false, passwordFile, "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner, "Should load with file password")

		if blsSigner != nil {
			loadedPubKey, err := blsSigner.BlsPubKey()
			assert.NoError(t, err)
			assert.Equal(t, privKey.PubKey().Bytes(), loadedPubKey.Bytes())
		}
	})

	t.Run("no-password mode for unencrypted key", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		blsKeyFile := filepath.Join(configDir, DefaultBlsKeyName)
		privKey := bls12381.GenPrivKey()
		bls := NewBls(privKey, blsKeyFile, "")
		bls.Key.Save("")

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should not be set for no-password test")

		blsSigner, err := LoadBlsSignerIfExists(tempDir, true, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner, "Should load unencrypted key with no-password mode")

		if blsSigner != nil {
			loadedPubKey, err := blsSigner.BlsPubKey()
			assert.NoError(t, err)
			assert.Equal(t, privKey.PubKey().Bytes(), loadedPubKey.Bytes())
		}
	})

	t.Run("return nil when key file doesn't exist", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		nonExistentKeyFile := filepath.Join(tempDir, "non-existent-key.json")

		blsSigner, err := LoadBlsSignerIfExists(tempDir, false, "", nonExistentKeyFile)
		assert.NoError(t, err)
		assert.Nil(t, blsSigner, "Should return nil when key file doesn't exist")
	})

	t.Run("validate multiple password methods error", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		passwordFile := filepath.Join(tempDir, "password.txt")
		err := os.WriteFile(passwordFile, []byte("test-password"), 0600)
		assert.NoError(t, err)

		t.Setenv(BlsPasswordEnvVar, "env-password")

		err = ValidatePasswordMethods(false, passwordFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multiple password methods detected")

		err = ValidatePasswordMethods(true, passwordFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multiple password methods detected")

		os.Unsetenv(BlsPasswordEnvVar)
		err = ValidatePasswordMethods(true, passwordFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multiple password methods detected")

		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("use custom key path", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		customKeyFile := filepath.Join(tempDir, "custom", "path", "custom_key.json")
		err := os.MkdirAll(filepath.Dir(customKeyFile), 0700)
		assert.NoError(t, err)

		testPassword := "custom-key-password"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		privKey := bls12381.GenPrivKey()
		bls := NewBls(privKey, customKeyFile, "")
		bls.Key.Save(testPassword)

		blsSigner, err := LoadBlsSignerIfExists(tempDir, false, "", customKeyFile)
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		if blsSigner != nil {
			loadedPubKey, err := blsSigner.BlsPubKey()
			assert.NoError(t, err)
			assert.Equal(t, privKey.PubKey().Bytes(), loadedPubKey.Bytes())
		}

		os.Unsetenv(BlsPasswordEnvVar)
	})
}

func TestValidatePasswordMethods(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("single password method - no password", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should not be set")

		err := ValidatePasswordMethods(true, "")
		assert.NoError(t, err)
	})

	t.Run("single password method - env var", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		t.Setenv(BlsPasswordEnvVar, "password")

		err := ValidatePasswordMethods(false, "")
		assert.NoError(t, err)

		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("multiple methods - no password and env var", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		t.Setenv(BlsPasswordEnvVar, "env-password")

		err := ValidatePasswordMethods(true, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multiple password methods detected")

		os.Unsetenv(BlsPasswordEnvVar)
	})
}

func TestGetBlsKeyPassword(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("env var password", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		testPassword := "test-env-var-password"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		password, err := GetBlsKeyPassword(false, "", false)
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)

		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("password file", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for password file test")

		tempDir := t.TempDir()
		passwordFile := filepath.Join(tempDir, "password.txt")
		testPassword := "test-file-password"
		err := os.WriteFile(passwordFile, []byte(testPassword), 0600)
		assert.NoError(t, err)

		password, err := GetBlsKeyPassword(false, passwordFile, false)
		assert.NoError(t, err)
		assert.Equal(t, testPassword, password)
	})

	t.Run("env var takes precedence over file", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		envPassword := "env-password-precedence"
		t.Setenv(BlsPasswordEnvVar, envPassword)

		tempDir := t.TempDir()
		passwordFile := filepath.Join(tempDir, "password.txt")
		filePassword := "file-password"
		err := os.WriteFile(passwordFile, []byte(filePassword), 0600)
		assert.NoError(t, err)

		password, err := GetBlsKeyPassword(false, "", false)
		assert.NoError(t, err)
		assert.Equal(t, envPassword, password)

		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("no password mode", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for no-password mode test")

		password, err := GetBlsKeyPassword(true, "", false)
		assert.NoError(t, err)
		assert.Equal(t, "", password)
	})
}

func TestShowBlsKey(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("show key with password file", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for password file test")

		tempDir := t.TempDir()

		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		keyPassword := "show-key-password"
		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		privKey := bls12381.GenPrivKey()
		blsKey := NewBls(privKey, keyFile, "")
		blsKey.Key.Save(keyPassword)

		passwordFile := filepath.Join(tempDir, "password.txt")
		err = os.WriteFile(passwordFile, []byte(keyPassword), 0600)
		assert.NoError(t, err)

		keyInfo, err := ShowBlsKey(keyFile, keyPassword)
		assert.NoError(t, err)
		assert.NotNil(t, keyInfo)
		assert.Equal(t, privKey.PubKey().Bytes(), keyInfo["pubkey"])
	})

	t.Run("show key with no-password mode", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for no-password test")

		tempDir := t.TempDir()

		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		privKey := bls12381.GenPrivKey()
		blsKey := NewBls(privKey, keyFile, "")
		blsKey.Key.Save("")

		keyInfo, err := ShowBlsKey(keyFile, "")
		assert.NoError(t, err)
		assert.NotNil(t, keyInfo)
		assert.Equal(t, privKey.PubKey().Bytes(), keyInfo["pubkey"])
	})

	t.Run("show key with multiple password methods fails", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()

		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		keyPassword := "show-key-password"
		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		blsKey := GenBls(keyFile, "", keyPassword)
		assert.NotNil(t, blsKey)

		t.Setenv(BlsPasswordEnvVar, keyPassword)

		passwordFile := filepath.Join(tempDir, "password.txt")
		err = os.WriteFile(passwordFile, []byte(keyPassword), 0600)
		assert.NoError(t, err)

		_, err = GetBlsKeyPassword(false, passwordFile, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multiple password methods detected")

		os.Unsetenv(BlsPasswordEnvVar)
	})
}

func TestCreateBlsKey(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("create key with no password", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for no-password test")

		tempDir := t.TempDir()
		blsKeyFile, _ := GetBlsKeyFileIfExist(tempDir, "")

		err := CreateBlsKey(blsKeyFile, "", "", testCmd)
		assert.NoError(t, err)

		configDir := filepath.Join(tempDir, "config")
		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		_, err = os.Stat(keyFile)
		assert.NoError(t, err, "Key file should exist")

		blsSigner, err := LoadBlsSignerIfExists(tempDir, true, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)
	})

	t.Run("create key with password from env var", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		blsKeyFile, _ := GetBlsKeyFileIfExist(tempDir, "")

		testPassword := "env-var-create-password"
		t.Setenv(BlsPasswordEnvVar, testPassword)

		err := CreateBlsKey(blsKeyFile, testPassword, "", testCmd)
		assert.NoError(t, err)

		configDir := filepath.Join(tempDir, "config")
		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		_, err = os.Stat(keyFile)
		assert.NoError(t, err, "Key file should exist")

		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("create key with password file", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for password file test")

		tempDir := t.TempDir()
		blsKeyFile, _ := GetBlsKeyFileIfExist(tempDir, "")

		passwordFile := filepath.Join(tempDir, "password.txt")
		testPassword := "file-create-password"
		err := os.WriteFile(passwordFile, []byte(testPassword), 0600)
		assert.NoError(t, err)

		configDir := filepath.Join(tempDir, "config")
		err = os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		err = CreateBlsKey(blsKeyFile, testPassword, passwordFile, testCmd)
		assert.NoError(t, err)

		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		_, err = os.Stat(keyFile)
		assert.NoError(t, err, "Key file should exist")
	})

	t.Run("error with key already exists - preserves existing key", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()
		blsKeyFile, _ := GetBlsKeyFileIfExist(tempDir, "")

		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0700)
		assert.NoError(t, err)

		firstPassword := ""
		err = CreateBlsKey(blsKeyFile, firstPassword, "", testCmd)
		assert.NoError(t, err)

		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		fileInfoBefore, err := os.Stat(keyFile)
		assert.NoError(t, err)
		modTimeBefore := fileInfoBefore.ModTime()

		time.Sleep(10 * time.Millisecond)

		fileInfoAfter, err := os.Stat(keyFile)
		assert.NoError(t, err)
		assert.Equal(t, modTimeBefore, fileInfoAfter.ModTime(), "Key file should not be modified")

		blsSigner, err := LoadBlsSignerIfExists(tempDir, true, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner, "Should be able to load with original password")
	})

	t.Run("fails with multiple password methods", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		tempDir := t.TempDir()

		t.Setenv(BlsPasswordEnvVar, "env-password")

		passwordFile := filepath.Join(tempDir, "password.txt")
		err := os.WriteFile(passwordFile, []byte("file-password"), 0600)
		assert.NoError(t, err)

		_, err = GetBlsKeyPassword(false, passwordFile, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multiple password methods detected")

		os.Unsetenv(BlsPasswordEnvVar)
	})
}

func TestUpdateBlsPassword(t *testing.T) {
	origEnvValue := os.Getenv(BlsPasswordEnvVar)
	defer t.Setenv(BlsPasswordEnvVar, origEnvValue)

	t.Run("create key with no password and update password from env var", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for no-password test")

		tempDir := t.TempDir()
		blsKeyFile, _ := GetBlsKeyFileIfExist(tempDir, "")

		err := CreateBlsKey(blsKeyFile, "", "", testCmd)
		assert.NoError(t, err)

		configDir := filepath.Join(tempDir, "config")
		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		_, err = os.Stat(keyFile)
		assert.NoError(t, err, "Key file should exist")

		blsSigner, err := LoadBlsSignerIfExists(tempDir, true, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		blsPrivKey, err := LoadBlsPrivKeyFromFile(blsKeyFile, "")
		assert.NoError(t, err)

		newPassword := "env-var-new-password"
		t.Setenv(BlsPasswordEnvVar, newPassword)

		err = UpdateBlsPassword(blsKeyFile, blsPrivKey, newPassword, "", testCmd)
		assert.NoError(t, err)

		blsSigner, err = LoadBlsSignerIfExists(tempDir, false, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		os.Unsetenv(BlsPasswordEnvVar)
	})

	t.Run("create key with no password and update password from password file", func(t *testing.T) {
		os.Unsetenv(BlsPasswordEnvVar)

		_, exists := os.LookupEnv(BlsPasswordEnvVar)
		assert.False(t, exists, "Environment variable should be unset for no-password test")

		tempDir := t.TempDir()

		blsKeyFile, _ := GetBlsKeyFileIfExist(tempDir, "")

		err := CreateBlsKey(blsKeyFile, "", "", testCmd)
		assert.NoError(t, err)

		configDir := filepath.Join(tempDir, "config")
		keyFile := filepath.Join(configDir, DefaultBlsKeyName)
		_, err = os.Stat(keyFile)
		assert.NoError(t, err, "Key file should exist")

		blsSigner, err := LoadBlsSignerIfExists(tempDir, true, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)

		blsPrivKey, err := LoadBlsPrivKeyFromFile(blsKeyFile, "")
		assert.NoError(t, err)

		newPassword := "env-var-new-password"
		newPasswordFile := filepath.Join(tempDir, "password.txt")
		err = os.WriteFile(newPasswordFile, []byte(newPassword), 0600)
		assert.NoError(t, err)

		err = UpdateBlsPassword(blsKeyFile, blsPrivKey, newPassword, newPasswordFile, testCmd)
		assert.NoError(t, err)

		blsSigner, err = LoadBlsSignerIfExists(tempDir, false, newPasswordFile, "")
		assert.NoError(t, err)
		assert.NotNil(t, blsSigner)
	})
}
