package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	t.Run("file not found", func(t *testing.T) {
		err := migrate(tempDir, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "priv_validator_key.json of previous version not found")
	})

	t.Run("invalid json format", func(t *testing.T) {
		// Create invalid json file
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		pvKeyFile := filepath.Join(configDir, "priv_validator_key.json")
		err = os.WriteFile(pvKeyFile, []byte("invalid json"), 0644)
		require.NoError(t, err)

		err = migrate(tempDir, "")
		require.Error(t, err)
	})

	t.Run("missing keys", func(t *testing.T) {
		// Create json file with missing keys
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		pvKeyFile := filepath.Join(configDir, "priv_validator_key.json")
		pvKey := PrevWrappedFilePV{}
		jsonBytes, err := cmtjson.MarshalIndent(pvKey, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(pvKeyFile, jsonBytes, 0644)
		require.NoError(t, err)

		err = migrate(tempDir, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not contain both the comet and bls keys")
	})

	t.Run("successful migration", func(t *testing.T) {
		// Create valid priv_validator_key.json
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		dataDir := filepath.Join(tempDir, "data")
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)

		pvKeyFile := filepath.Join(configDir, "priv_validator_key.json")
		pvKey := PrevWrappedFilePV{
			PrivKey:    ed25519.GenPrivKey(),
			BlsPrivKey: bls12381.GenPrivKey(),
		}
		jsonBytes, err := cmtjson.MarshalIndent(pvKey, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(pvKeyFile, jsonBytes, 0644)
		require.NoError(t, err)

		// Run migration
		err = migrate(tempDir, "testpassword")
		require.NoError(t, err)

		// Check if new files are created
		newPvKeyFile := filepath.Join(configDir, "priv_validator_key.json")
		newPvStateFile := filepath.Join(dataDir, "priv_validator_state.json")
		newBlsKeyFile := filepath.Join(configDir, "bls_key.json")
		newBlsPasswordFile := filepath.Join(configDir, "bls_password.txt")
		require.FileExists(t, newPvKeyFile)
		require.FileExists(t, newPvStateFile)
		require.FileExists(t, newBlsKeyFile)
		require.FileExists(t, newBlsPasswordFile)

		t.Run("verify separated files", func(t *testing.T) {
			verifySeparateFiles(
				newPvKeyFile, newPvStateFile, newBlsKeyFile, newBlsPasswordFile,
				pvKey.PrivKey, pvKey.BlsPrivKey,
			)
		})
	})
}

func TestLoadPrevWrappedFilePV(t *testing.T) {
	tempDir := t.TempDir()
	pvKeyFile := filepath.Join(tempDir, "priv_validator_key.json")

	t.Run("file not found", func(t *testing.T) {
		_, err := loadPrevWrappedFilePV(pvKeyFile)
		require.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		err := os.WriteFile(pvKeyFile, []byte("invalid json"), 0644)
		require.NoError(t, err)

		_, err = loadPrevWrappedFilePV(pvKeyFile)
		require.Error(t, err)
	})

	t.Run("valid file", func(t *testing.T) {
		pvKey := PrevWrappedFilePV{
			PrivKey:    ed25519.GenPrivKey(),
			BlsPrivKey: bls12381.GenPrivKey(),
		}
		jsonBytes, err := cmtjson.MarshalIndent(pvKey, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(pvKeyFile, jsonBytes, 0644)
		require.NoError(t, err)

		loadedPvKey, err := loadPrevWrappedFilePV(pvKeyFile)
		require.NoError(t, err)
		require.NotNil(t, loadedPvKey.PrivKey)
		require.NotNil(t, loadedPvKey.BlsPrivKey)
	})
}
