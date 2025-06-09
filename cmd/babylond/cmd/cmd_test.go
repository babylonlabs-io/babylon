package cmd_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/cmd/babylond/cmd"
	"github.com/cosmos/cosmos-sdk/client/flags"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
)

func TestInitCmd(t *testing.T) {
	tmpDir := t.TempDir()
	rootCmd := cmd.NewRootCmd()
	rootCmd.SetArgs([]string{
		"init",     // Test the init cmd
		"app-test", // Moniker
		fmt.Sprintf("--%s=%s", cli.FlagOverwrite, "true"), // Overwrite genesis.json, in case it already exists
		fmt.Sprintf("--%s=%s", flags.FlagHome, tmpDir),
		"--no-bls-password",
	})

	require.NoError(t, svrcmd.Execute(rootCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome))
}
