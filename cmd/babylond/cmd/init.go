package cmd

import (
	"fmt"

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
			if err := cosmosInitCmd.RunE(cmd, args); err != nil {
				return fmt.Errorf("failed to run init command: %w", err)
			}

			homeDir, _ := cmd.Flags().GetString(flags.FlagHome)
			password, _ := cmd.Flags().GetString(flagBlsPassword)
			createBlsKeyAndSave(homeDir, password)
			return nil
		},
	}
	cmd.Flags().AddFlagSet(cosmosInitCmd.Flags())
	cmd.Flags().String(flagBlsPassword, "", "The password for the BLS key. If the flag is not set, the password will be read from the prompt.")
	return cmd
}
