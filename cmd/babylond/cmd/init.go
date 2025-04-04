package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	appsigner "github.com/babylonlabs-io/babylon/app/signer"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/types/module"
	genutil "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
)

// InitCmd returns the command to initialize the config.
// It runs InitCmd of cosmos-sdk first, then runs createBlsKeyAndSave.
func InitCmd(mbm module.BasicManager, defaultNodeHome string) *cobra.Command {
	cosmosInitCmd := genutil.InitCmd(mbm, defaultNodeHome)
	cmd := &cobra.Command{
		Use:   cosmosInitCmd.Use,
		Short: cosmosInitCmd.Short,
		Long: `Initializes the configuration files for the validator and node.
		 This command also asks for a password to 
		 generate the BLS key and encrypt it into an erc2335 structure.`,
		Args: cosmosInitCmd.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			// run cosmos init first
			if err := cosmosInitCmd.RunE(cmd, args); err != nil {
				return fmt.Errorf("failed to run init command: %w", err)
			}

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

			var customKeyFile string
			if _, err := os.Stat(configFilePath); err == nil {
				// Only attempt to read from app.toml if it exists
				v := viper.New()
				v.SetConfigFile(configFilePath)
				if err := v.ReadInConfig(); err == nil {
					// Successfully read config
					v.SetConfigName("app")
					customKeyFile = cast.ToString(v.Get("bls-config.bls-key-file"))
				}
			}

			// Determine password at the system boundary
			password, err := appsigner.GetBlsKeyPassword(noBlsPassword, passwordFile, true)
			if err != nil {
				return fmt.Errorf("failed to determine BLS password: %w", err)
			}

			// Generate BLS key using the refactored function with explicit password and custom key file path
			if err := appsigner.CreateBlsKey(homeDir, password, passwordFile, customKeyFile, cmd); err != nil {
				return fmt.Errorf("failed to create BLS key: %w", err)
			}

			return nil
		},
	}
	cmd.Flags().AddFlagSet(cosmosInitCmd.Flags())
	cmd.Flags().Bool(flagNoBlsPassword, false, "The BLS key will use an empty password if the flag is set")
	cmd.Flags().String(flagBlsPasswordFile, "", "Path to a file to store the BLS password (not recommended)")
	return cmd
}
