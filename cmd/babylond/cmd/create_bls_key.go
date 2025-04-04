package cmd

import (
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	appsigner "github.com/babylonlabs-io/babylon/app/signer"
)

func CreateBlsKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-bls-key",
		Short: "Create a pair of BLS keys for a validator",
		Long: strings.TrimSpace(`create-bls will create a pair of BLS keys that are used to
send BLS signatures for checkpointing.

Password precedence:
1. Environment variable BABYLON_BLS_PASSWORD
2. Password specified with --insecure-bls-password flag 
3. Password file specified with --bls-password-file flag
4. Interactive prompt

Example:
$ babylond create-bls-key
$ babylond create-bls-key --bls-password-file=/path/to/password.txt
$ babylond create-bls-key --no-bls-password
`,
		),

		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, _ := cmd.Flags().GetString(flags.FlagHome)
			noBlsPassword, _ := cmd.Flags().GetBool(flagNoBlsPassword)
			explicitPassword, _ := cmd.Flags().GetString(flagInsecureBlsPassword)
			passwordFile, _ := cmd.Flags().GetString(flagBlsPasswordFile)

			// Generate BLS key using the common helper function
			return appsigner.CreateBlsKey(homeDir, noBlsPassword, explicitPassword, passwordFile)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "Generate BLS key without password protection")
	cmd.Flags().String(flagInsecureBlsPassword, "", "The password for the BLS key. If not set, will try env var, then prompt")
	cmd.Flags().String(flagBlsPasswordFile, "", "Custom file path to store the BLS password")
	return cmd
}
