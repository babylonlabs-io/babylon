package cmd

import (
	"fmt"

	appsigner "github.com/babylonlabs-io/babylon/app/signer"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
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
			noBlsPassword, _ := cmd.Flags().GetBool(flagNoBlsPassword)

			blsKeyFile := appsigner.DefaultBlsKeyFile(homeDir)
			blsPasswordFile := appsigner.DefaultBlsPasswordFile(homeDir)

			if err := appsigner.EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
				return fmt.Errorf("failed to ensure dirs exist: %w", err)
			}

			if noBlsPassword {
				// We still create an empty password file for backward compatibility
				// This ensures that the system components that expect this file to exist will still work
				// even when no password protection is used
				bls := appsigner.NewBls(bls12381.GenPrivKey(), blsKeyFile, blsPasswordFile)
				bls.Key.Save("")
				fmt.Printf("BLS key generated successfully without password protection.\n")
				fmt.Printf("Note: An empty password file has been created at %s for backward compatibility.\n", blsPasswordFile)
				return nil
			}

			password, _ := cmd.Flags().GetString(flagInsecureBlsPassword)
			if password == "" {
				password = appsigner.NewBlsPassword()
			}

			// We deliberately pass an empty string for the password file path ("") to avoid
			// automatically creating a password file. This gives operators full control over
			// how they want to store and provide the password (env var or custom password file).
			// Security best practice is to not store the password on disk at all and use the
			// environment variable instead.
			bls := appsigner.NewBls(bls12381.GenPrivKey(), blsKeyFile, "")
			bls.Key.Save(password)

			fmt.Printf("\nIMPORTANT: Your BLS key has been created with password protection.\n")
			fmt.Printf("You must provide this password when starting the node using one of these methods:\n")
			fmt.Printf("1. (Recommended) Set the BABYLON_BLS_PASSWORD environment variable:\n")
			fmt.Printf("export %s=<your_password>\n", appsigner.BlsPasswordEnvVar)
			fmt.Printf("2. (Not recommended) Create a password file and provide its path when starting the node:\n")
			fmt.Printf("babylond start --bls-password-file=<path_to_file>\n")
			fmt.Printf("\nRemember to securely store your password. If you lose it, you won't be able to access your BLS key.\n")

			return nil
		},
	}
	cmd.Flags().AddFlagSet(cosmosInitCmd.Flags())
	cmd.Flags().String(flagInsecureBlsPassword, "", "The password for the BLS key. If the flag is not set, the password will be read from the prompt.")
	cmd.Flags().Bool(flagNoBlsPassword, false, "The BLS key will use an empty password if the flag is set.")
	return cmd
}
