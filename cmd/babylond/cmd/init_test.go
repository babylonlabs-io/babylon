package cmd

import (
	"path/filepath"
	"testing"

	"github.com/babylonlabs-io/babylon/privval"
	cfg "github.com/cometbft/cometbft/config"
)

func TestSaveFiles(t *testing.T) {

	// clientCtx := client.GetClientContextFromCmd(&cobra.Command{})
	homeDir := "test"
	blsCfg := privval.BlsConfig{
		RootDir:         homeDir,
		BlsKeyPath:      filepath.Join(homeDir, cfg.DefaultConfigDir, privval.DefaultBlsKeyName),
		BlsPasswordPath: filepath.Join(homeDir, cfg.DefaultConfigDir, privval.DefaultBlsPasswordName),
	}
	t.Log(blsCfg)
}
