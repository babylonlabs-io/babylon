package cmd

import (
	"os"
	"path/filepath"
	"testing"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/stretchr/testify/require"
)

func TestGenerateBlsPop(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	t.Run("generate bls pop", func(t *testing.T) {
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		blsPrivKey := bls12381.GenPrivKey()
		cmtPrivKey := ed25519.GenPrivKey()

		// generate bls pop using generated keys,
		// comet private key, bls private key.
		// bls pop is saved to bls_pop.json in default config directory.
		err = generateBlsPop(tempDir, cmtPrivKey, blsPrivKey)
		require.NoError(t, err)

		// check if bls pop file exists
		newBlsPopFile := filepath.Join(configDir, appsigner.DefaultBlsPopName)
		require.FileExists(t, newBlsPopFile)

		// load bls pop from bls_pop.json
		loadBlsPop, err := appsigner.LoadBlsPop(newBlsPopFile)
		require.NoError(t, err)

		// create pop already created in generateBlsPop
		sampleBlsPop, err := appsigner.BuildPoP(cmtPrivKey, blsPrivKey)
		require.NoError(t, err)

		// check if bls pop is correct
		require.Equal(t, loadBlsPop.Pop, sampleBlsPop)
		require.Equal(t, loadBlsPop.BlsPubkey, blsPrivKey.PubKey())
	})
}
