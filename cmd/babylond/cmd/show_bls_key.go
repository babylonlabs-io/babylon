package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v4/app"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
)

// ShowBlsKeyCmd displays information about the BLS key
func ShowBlsKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-bls-key",
		Short: "Display information about the BLS key",
		Long: strings.TrimSpace(`Display information about the BLS key.

The command will try to load the existing BLS key and show its public key and other information.
Password precedence for decrypting the key:
1. Environment variable BABYLON_BLS_PASSWORD
2. Password file (from --bls-password-file or default location)
3. Prompt the user for password

Example:
$ babylond show-bls-key
$ babylond show-bls-key --bls-password-file=/path/to/password.txt
$ babylond show-bls-key --no-bls-password
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			blsKeyFile, exist := appsigner.GetBlsKeyFileIfExist(homeDir, "")
			if !exist {
				return fmt.Errorf("BLS key file does not exist at %s", blsKeyFile)
			}

			noBlsPassword, err := cmd.Flags().GetBool(flagNoBlsPassword)
			if err != nil {
				return fmt.Errorf("failed to get noBlsPassword flag: %w", err)
			}
			passwordFile, err := cmd.Flags().GetString(flagBlsPasswordFile)
			if err != nil {
				return fmt.Errorf("failed to get passwordFile flag: %w", err)
			}

			// Convert passwordFile to absolute path if it's not empty and not already absolute
			if passwordFile != "" && !filepath.IsAbs(passwordFile) {
				absPath, err := filepath.Abs(passwordFile)
				if err != nil {
					return fmt.Errorf("failed to resolve password file path: %w", err)
				}
				passwordFile = absPath
			}

			// Determine password at the system boundary
			password, err := appsigner.GetBlsKeyPassword(noBlsPassword, passwordFile, false)
			if err != nil {
				return fmt.Errorf("failed to determine BLS password: %w", err)
			}

			info, err := appsigner.ShowBlsKey(blsKeyFile, password)
			if err != nil {
				return fmt.Errorf("failed to show BLS key: %w", err)
			}

			jsonBytes, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal key info: %w", err)
			}

			cmd.Println(string(jsonBytes))
			return nil
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "Indicate that the BLS key has no password protection")
	cmd.Flags().String(flagBlsPasswordFile, "", "Path to a file containing the BLS password")

	return cmd
}
