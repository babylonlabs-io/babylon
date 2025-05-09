package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v2/app"
	appsigner "github.com/babylonlabs-io/babylon/v2/app/signer"
)

func UpdateBlsPasswordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-bls-password",
		Short: "Update the password of BLS key",
		Long: strings.TrimSpace(`update-bls-password will update the password of BLS key.

Password precedence:
1. Environment variable BABYLON_BLS_PASSWORD
2. Password file specified with --bls-password-file flag
3. Interactive prompt

Example:
$ babylond update-bls-password
$ babylond update-bls-password --bls-password-file=/path/to/password.txt
$ babylond update-bls-password --no-bls-password
`,
		),

		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			cmd.Println("\n ⚠️ IMPORTANT: To update the password of BLS key, you need to provide the old password. ⚠️")
			cmd.Println("Due to security issues, older versions only accept passwords via environment variables or prompts.")
			cmd.Println("If the previous bls key has no password, enter a blank string value at the prompt.")
			cmd.Println("If this command uses both old and new passwords from environment variables, set different values for each:")
			cmd.Println("- old password: BABYLON_OLD_BLS_PASSWORD")
			cmd.Println("- new password: BABYLON_BLS_PASSWORD")

			newBlsPassword, newFound := appsigner.GetBlsPasswordFromEnv()

			// Since only BABYLON_BLS_PASSWORD is used to get password,
			// set old password to BABYLON_BLS_PASSWORD.
			oldBlsPassword, oldFound := appsigner.GetOldBlsPasswordFromEnv()
			if oldFound {
				os.Setenv(appsigner.BlsPasswordEnvVar, oldBlsPassword)
			}

			oldPassword, err := appsigner.GetBlsKeyPassword(false, "", false)
			if err != nil {
				return fmt.Errorf("failed to get old BLS password: %w", err)
			}

			blsPrivKey, err := appsigner.LoadBlsPrivKey(homeDir, oldPassword)
			if err != nil {
				return fmt.Errorf("failed to load BLS key: %w", err)
			}

			cmd.Println("\n Update the existing bls key with a new password.")
			cmd.Println("The new password can be managed in the same way as the create-bls-key cmd")

			// If new BLS password is set in environment variable,
			// set new password to BABYLON_BLS_PASSWORD.
			if newFound {
				os.Setenv(appsigner.BlsPasswordEnvVar, newBlsPassword)
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
			return appsigner.UpdateBlsPassword(homeDir, blsPrivKey, password, passwordFile, cmd)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "Generate BLS key without password protection")
	cmd.Flags().String(flagBlsPasswordFile, "", "Custom file path to store the BLS password")
	return cmd
}
