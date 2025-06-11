package cmd

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v3/app"
	appsigner "github.com/babylonlabs-io/babylon/v3/app/signer"
)

func CreateBlsKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-bls-key",
		Short: "Create a pair of BLS keys for a validator",
		Long: strings.TrimSpace(`create-bls will create a pair of BLS keys that are used to
send BLS signatures for checkpointing.

Password precedence:
1. Environment variable BABYLON_BLS_PASSWORD
2. Password file specified with --bls-password-file flag
3. Interactive prompt

Example:
$ babylond create-bls-key
$ babylond create-bls-key --bls-password-file=/path/to/password.txt
$ babylond create-bls-key --no-bls-password
`,
		),

		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			noBlsPassword, err := cmd.Flags().GetBool(flagNoBlsPassword)
			if err != nil {
				return fmt.Errorf("failed to get noBlsPassword flag: %w", err)
			}
			passwordFile, err := cmd.Flags().GetString(flagBlsPasswordFile)
			if err != nil {
				return fmt.Errorf("failed to get passwordFile flag: %w", err)
			}

			// Determine password at the system boundary
			password, err := appsigner.GetBlsKeyPassword(noBlsPassword, passwordFile, true)
			if err != nil {
				return fmt.Errorf("failed to determine BLS password: %w", err)
			}

			// Generate BLS key using the refactored function with explicit password
			return appsigner.CreateBlsKey(homeDir, password, passwordFile, cmd)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "Generate BLS key without password protection")
	cmd.Flags().String(flagBlsPasswordFile, "", "Custom file path to store the BLS password")
	return cmd
}
