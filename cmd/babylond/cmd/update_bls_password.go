package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v3/app"
	appsigner "github.com/babylonlabs-io/babylon/v3/app/signer"
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

			cmd.Println("\n⚠️ IMPORTANT: Your BLS key file will be overwritten! ⚠️")
			cmd.Println("1. (Recommended) Please make a backup of your BLS key before proceeding.")
			cmd.Println("2. To update the password of BLS key, you need to provide the old password.")
			cmd.Println("3. Due to security issues, older versions only accept passwords via environment variables or prompts.")
			cmd.Println("4. If the previous BLS key has no password, enter a blank string value at the prompt.")
			cmd.Println("5. If BLS password is used from environment variable, you should reset environment variable to the new password after updating.")

			// Ask for confirmation before proceeding
			fmt.Print("\nDo you want to continue with updating the BLS password? [y/n]: ")
			var response string
			_, err = fmt.Scanln(&response)
			if err != nil {
				// If there's an error reading input, default to 'n' for safety
				return fmt.Errorf("failed to read user input: %w", err)
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				return fmt.Errorf("BLS password update cancelled by user")
			}

			// Get old password from either environment variable or prompt
			oldPassword, err := appsigner.GetBlsKeyPassword(false, "", false)
			if err != nil {
				return fmt.Errorf("failed to get old BLS password: %w", err)
			}

			// Unset environment variable for old version of password
			// to prevent it from being used in the next step
			// User should reset environment variable to the new password after updating
			if err := os.Unsetenv(appsigner.BlsPasswordEnvVar); err != nil {
				return fmt.Errorf("failed to unset BLS password environment variable: %w", err)
			}

			blsPrivKey, err := appsigner.LoadBlsPrivKey(homeDir, oldPassword)
			if err != nil {
				return fmt.Errorf("failed to load BLS key: %w", err)
			}

			cmd.Println("\nUpdate the existing bls key with a new password.")

			noBlsPassword, err := cmd.Flags().GetBool(flagNoBlsPassword)
			if err != nil {
				return fmt.Errorf("failed to get noBlsPassword flag: %w", err)
			}
			passwordFile, err := cmd.Flags().GetString(flagBlsPasswordFile)
			if err != nil {
				return fmt.Errorf("failed to get passwordFile flag: %w", err)
			}

			// Determine password at the system boundary
			newPassword, err := appsigner.GetBlsKeyPassword(noBlsPassword, passwordFile, true)
			if err != nil {
				return fmt.Errorf("failed to determine new BLS password: %w", err)
			}

			// Generate BLS key using the refactored function with explicit password
			return appsigner.UpdateBlsPassword(homeDir, blsPrivKey, newPassword, passwordFile, cmd)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "Generate BLS key without password protection")
	cmd.Flags().String(flagBlsPasswordFile, "", "Custom file path to store the BLS password")
	return cmd
}
