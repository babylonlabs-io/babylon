package privval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/test-go/testify/assert"
)

func TestNewBlsPV(t *testing.T) {
	pv := NewBlsPV(bls12381.GenPrivKey(), "test")
	assert.NotNil(t, pv)
}

func TestCleanUp(t *testing.T) {
	blsKeyFilePath := DefaultBlsConfig().BlsKeyFile()
	t.Log("bls key file path", blsKeyFilePath)
	cleanup(blsKeyFilePath)
}

func TestLoadOrGenBlsPV(t *testing.T) {

	t.Run("clean file path", func(t *testing.T) {
		blsKeyFilePath := DefaultBlsConfig().BlsKeyFile()
		t.Log("bls key file path", blsKeyFilePath)
		cleanup(blsKeyFilePath)
	})

	t.Run("set default config", func(t *testing.T) {
		blsCfg := DefaultBlsConfig()
		assert.NotNil(t, blsCfg)
		assert.Equal(t, blsCfg.RootDir, cmtcfg.DefaultDataDir)
		assert.Equal(t, blsCfg.BlsKeyPath, filepath.Join(cmtcfg.DefaultDataDir, DefaultBlsKeyName))

		t.Run("generate key without mnemonic", func(t *testing.T) {
			blsPubKey, err := InitializeNodeValidatorBlsFiles(&blsCfg, "password")
			assert.NoError(t, err)
			assert.NotNil(t, blsPubKey)
		})

		t.Run("load key with password", func(t *testing.T) {
			blsPubKey, err := InitializeNodeValidatorBlsFiles(&blsCfg, "password")
			assert.NoError(t, err)
			assert.NotNil(t, blsPubKey)
		})
	})
}

func cleanup(blsKeyPath string) {
	_ = os.RemoveAll(filepath.Dir(blsKeyPath))
}
