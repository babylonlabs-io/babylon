package cmd

import (
	"fmt"

	appsigner "github.com/babylonlabs-io/babylon/app/signer"
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

			homeDir, _ := cmd.Flags().GetString(flags.FlagHome)
			appsigner.GenBls(appsigner.DefaultBlsKeyFile(homeDir), appsigner.DefaultBlsPasswordFile(homeDir), blsPassword(cmd))
			return nil
		},
	}
	cmd.Flags().AddFlagSet(cosmosInitCmd.Flags())
	cmd.Flags().String(flagBlsPassword, "", "The password for the BLS key. If the flag is not set, the password will be read from the prompt.")
	cmd.Flags().Bool(flagNoBlsPassword, false, "The BLS key will use an empty password if the flag is set.")
	return cmd
}
