package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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

			// Read app.toml for BLS key file path
			configDir := filepath.Join(homeDir, "config")
			configFilePath := filepath.Join(configDir, "app.toml")
			if _, err := os.Stat(configFilePath); err == nil {
				// Only attempt to read from app.toml if it exists
				v := viper.New()
				v.SetConfigFile(configFilePath)
				if err := v.ReadInConfig(); err == nil {
					// Successfully read config
					v.SetConfigName("app")
				}
				customKeyFile := cast.ToString(v.Get("bls-config.bls-key-file"))

				// Determine password at the system boundary
				password, err := appsigner.GetBlsKeyPassword(noBlsPassword, passwordFile, true)
				if err != nil {
					return fmt.Errorf("failed to determine BLS password: %w", err)
				}

				// Generate BLS key using the refactored function with explicit password and custom key file path
				return appsigner.CreateBlsKey(homeDir, password, passwordFile, customKeyFile, cmd)
			}

			// If app.toml doesn't exist, continue with default key path
			password, err := appsigner.GetBlsKeyPassword(noBlsPassword, passwordFile, true)
			if err != nil {
				return fmt.Errorf("failed to determine BLS password: %w", err)
			}

			// Generate BLS key using the refactored function with explicit password
			return appsigner.CreateBlsKey(homeDir, password, passwordFile, "", cmd)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "Generate BLS key without password protection")
	cmd.Flags().String(flagBlsPasswordFile, "", "Custom file path to store the BLS password")
	return cmd
}
