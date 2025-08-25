package cmd

import (
	"fmt"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/spf13/cobra"

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

			// Check bls_key.json is already exist
			blsKeyFile, exist := appsigner.GetBlsKeyFileIfExist(homeDir, "")
			if exist {
				return fmt.Errorf("BLS key already exists at %s. If you need to generate a new key, please manually delete the existing file first", blsKeyFile)
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
			if err := appsigner.CreateBlsKey(blsKeyFile, password, passwordFile, cmd); err != nil {
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
