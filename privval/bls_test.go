package privval

import (
	"os"
	"testing"

	"github.com/cometbft/cometbft/crypto/ed25519"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/test-go/testify/assert"
)

func TestNewBlsPV(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	keyFilePath := DefaultBlsKeyFile(tempDir)
	passwordFilePath := DefaultBlsPasswordFile(tempDir)

	err := EnsureDirs(keyFilePath, passwordFilePath)
	assert.NoError(t, err)

	t.Run("save bls key to file without delegator address", func(t *testing.T) {
		pv := NewBlsPV(bls12381.GenPrivKey(), keyFilePath, passwordFilePath, "")
		assert.NotNil(t, pv)

		password := "password"
		pv.Key.Save(password, "")

		t.Run("load bls key from file", func(t *testing.T) {
			loadedPv := LoadBlsPV(keyFilePath, passwordFilePath)
			assert.NotNil(t, loadedPv)

			assert.Equal(t, pv.Key.PrivKey, loadedPv.Key.PrivKey)
			assert.Equal(t, pv.Key.PubKey.Bytes(), loadedPv.Key.PubKey.Bytes())
		})
	})

	t.Run("save bls key to file with delegator address", func(t *testing.T) {
		pv := NewBlsPV(bls12381.GenPrivKey(), keyFilePath, passwordFilePath, "")
		assert.NotNil(t, pv)

		password := "password"

		delegatorAddress := types.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String()
		pv.Key.Save(password, delegatorAddress)

		t.Run("load bls key from file", func(t *testing.T) {
			loadedPv := LoadBlsPV(keyFilePath, passwordFilePath)
			assert.NotNil(t, loadedPv)

			assert.Equal(t, pv.Key.PrivKey, loadedPv.Key.PrivKey)
			assert.Equal(t, pv.Key.PubKey.Bytes(), loadedPv.Key.PubKey.Bytes())
			assert.Equal(t, delegatorAddress, loadedPv.Key.DelegatorAddress)
		})
	})
}
