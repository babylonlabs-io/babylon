package cmd_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/app/signer"
	"github.com/babylonlabs-io/babylon/cmd/babylond/cmd"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	stakingcli "github.com/cosmos/cosmos-sdk/x/staking/client/cli"
)

func TestInitCmd(t *testing.T) {
	rootCmd := cmd.NewRootCmd()
	rootCmd.SetArgs([]string{
		"init",     // Test the init cmd
		"app-test", // Moniker
		fmt.Sprintf("--%s=%s", cli.FlagOverwrite, "true"), // Overwrite genesis.json, in case it already exists
		fmt.Sprintf("--%s=%s", "bls-password", "testpassword"),
	})

	require.NoError(t, svrcmd.Execute(rootCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome))
}

func TestStartCmd(t *testing.T) {
	err := os.RemoveAll(app.DefaultNodeHome)
	require.NoError(t, err)

	tempApp := app.NewTmpBabylonApp()

	rootCmd := cmd.NewRootCmd()
	rootCmd.SetArgs([]string{
		"init",
		"app-test",
		fmt.Sprintf("--%s=%s", cli.FlagOverwrite, "true"),
		fmt.Sprintf("--%s=%s", "bls-password", "testpassword"),
	})

	require.NoError(t, svrcmd.Execute(rootCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome))

	blsKeyFile := signer.DefaultBlsKeyFile(app.DefaultNodeHome)
	blsPasswordFile := signer.DefaultBlsPasswordFile(app.DefaultNodeHome)
	require.FileExists(t, blsKeyFile, "BLS key file should exist after init")
	require.FileExists(t, blsPasswordFile, "BLS password file should exist after init")

	keyringDir := filepath.Join(app.DefaultNodeHome, "keyring-test")
	err = os.MkdirAll(keyringDir, 0o755)
	require.NoError(t, err)

	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, app.DefaultNodeHome, nil, tempApp.AppCodec())
	require.NoError(t, err)

	keyInfo, _, err := kb.NewMnemonic(
		"validator",
		keyring.English,
		sdk.FullFundraiserPath,
		keyring.DefaultBIP39Passphrase,
		hd.Secp256k1,
	)
	require.NoError(t, err)

	addr, err := keyInfo.GetAddress()
	require.NoError(t, err)

	prepareCmd := cmd.NewRootCmd()
	prepareCmd.SetArgs([]string{
		"prepare-genesis",
		"testnet",
		"test-1",
	})
	require.NoError(t, svrcmd.Execute(prepareCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome))

	addAccountCmd := cmd.NewRootCmd()
	addAccountCmd.SetArgs([]string{
		"add-genesis-account",
		addr.String(),
		"2000000000000ubbn",
	})
	require.NoError(t, svrcmd.Execute(addAccountCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome))

	createValidatorCmd := cmd.NewRootCmd()
	createValidatorCmd.SetArgs([]string{
		"gentx",
		"validator",
		"1500000000000ubbn",
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "test-1"),
		fmt.Sprintf("--%s=%s", flags.FlagKeyringBackend, keyring.BackendTest),
		fmt.Sprintf("--%s=%s", stakingcli.FlagMoniker, "test-validator"),
		fmt.Sprintf("--%s=%s", flags.FlagHome, app.DefaultNodeHome),
		fmt.Sprintf("--%s=%s", flags.FlagFees, "2000ubbn"),
	})
	require.NoError(t, svrcmd.Execute(createValidatorCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome))

	collectGentxsCmd := cmd.NewRootCmd()
	collectGentxsCmd.SetArgs([]string{
		"collect-gentxs",
	})
	require.NoError(t, svrcmd.Execute(collectGentxsCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome))

	// Now that we have the validator setup complete, delete the BLS files
	// to test that they get recreated
	err = os.Remove(blsKeyFile)
	require.NoError(t, err)
	err = os.Remove(blsPasswordFile)
	require.NoError(t, err)

	require.NoFileExists(t, blsKeyFile, "BLS key file should be deleted")
	require.NoFileExists(t, blsPasswordFile, "BLS password file should be deleted")

	startCmd := cmd.NewRootCmd()
	startCmd.SetArgs([]string{
		"start",
		"--wasm.skip_wasmvm_version_check=true",
	})

	done := make(chan struct{})

	go func() {
		err = svrcmd.Execute(startCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome)
		require.NoError(t, err)
		close(done)
	}()

	time.Sleep(3 * time.Second)

	require.FileExists(t, blsKeyFile, "BLS key file should be recreated by start command")
	require.FileExists(t, blsPasswordFile, "BLS password file should be recreated by start command")

	blsSigner := signer.LoadBlsSignerIfExists(app.DefaultNodeHome)
	require.NotNil(t, blsSigner, "Should be able to load BLS signer after start")

	p, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, p.Signal(os.Interrupt))

	select {
	case <-done:
		// Chain has stopped
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for chain to stop")
	}
}
