package privval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/test-go/testify/assert"
)

// func TestNewBlsPV(t *testing.T) {
// 	pv := NewBlsPV(bls12381.GenPrivKey(), "test")
// 	assert.NotNil(t, pv)
// }

func TestCleanUp(t *testing.T) {
	blsKeyFilePath := DefaultBlsConfig().BlsKeyFile()
	t.Log("bls key file path", blsKeyFilePath)
	cleanup(blsKeyFilePath)
}

func TestInitializeBlsFile(t *testing.T) {

	t.Run("set default config", func(t *testing.T) {
		blsCfg := DefaultBlsConfig()
		assert.NotNil(t, blsCfg)
		assert.Equal(t, blsCfg.BlsKeyPath, filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsKeyName))

		password := erc2335.CreateRandomPassword()
		t.Log("password", password)

		t.Run("generate key without mnemonic", func(t *testing.T) {
			blsPubKey, err := InitializeBlsFile(&blsCfg, password)
			assert.NoError(t, err)
			assert.NotNil(t, blsPubKey)
		})

		t.Run("load key with password", func(t *testing.T) {
			blsPubKey, err := InitializeBlsFile(&blsCfg, password)
			assert.NoError(t, err)
			assert.NotNil(t, blsPubKey)
		})

		t.Run("clean file path", func(t *testing.T) {
			blsKeyFilePath := DefaultBlsConfig().BlsKeyFile()
			t.Log("bls key file path", blsKeyFilePath)
			cleanup(blsKeyFilePath)
		})
	})
}

func TestSavePasswordToFile(t *testing.T) {

	blsCfg := DefaultBlsConfig()

	t.Run("failed to load unsaved file", func(t *testing.T) {
		_, err := erc2335.LoadPaswordFromFile(blsCfg.BlsPasswordFile())
		assert.Error(t, err)
	})

	t.Run("create password file", func(t *testing.T) {
		password := erc2335.CreateRandomPassword()
		t.Log("password", password)

		err := os.MkdirAll(filepath.Dir(blsCfg.BlsPasswordFile()), 0o777)
		assert.NoError(t, err)

		err = erc2335.SavePasswordToFile(password, blsCfg.BlsPasswordFile())
		assert.NoError(t, err)

		t.Run("load password file", func(t *testing.T) {

			loadPassword, err := erc2335.LoadPaswordFromFile(blsCfg.BlsPasswordFile())
			assert.NoError(t, err)
			assert.Equal(t, password, loadPassword)
		})
	})

	t.Run("clean file path", func(t *testing.T) {
		blsPasswordFilePath := DefaultBlsConfig().BlsPasswordFile()
		t.Log("bls passwordd file path", blsPasswordFilePath)
		cleanup(blsPasswordFilePath)
	})
}

func cleanup(blsKeyPath string) {
	_ = os.RemoveAll(filepath.Dir(blsKeyPath))
}
