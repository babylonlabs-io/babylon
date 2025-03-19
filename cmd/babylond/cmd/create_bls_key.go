package cmd

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	appsigner "github.com/babylonlabs-io/babylon/app/signer"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
)

func CreateBlsKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-bls-key",
		Short: "Create a pair of BLS keys for a validator",
		Long: strings.TrimSpace(`create-bls will create a pair of BLS keys that are used to
send BLS signatures for checkpointing.

BLS keys are stored along with other validator keys in priv_validator_key.json,
which should exist before running the command (via babylond init or babylond testnet).

Example:
$ babylond create-bls-key --home ./
`,
		),

		RunE: func(cmd *cobra.Command, args []string) error {
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
				fmt.Printf("Note: An empty password file has been created for backward compatibility.\n")
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
			fmt.Printf("   export %s=<your_password>\n", appsigner.BlsPasswordEnvVar)
			fmt.Printf("2. (Not recommended) Create a password file and provide its path when starting the node:\n")
			fmt.Printf("   babylond start --bls-password-file=<path_to_file>\n")
			fmt.Printf("\nRemember to securely store your password. If you lose it, you won't be able to access your BLS key.\n")

			return nil
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().String(flagInsecureBlsPassword, "", "The password for the BLS key. If the flag is not set, the password will be read from the prompt.")
	cmd.Flags().Bool(flagNoBlsPassword, false, "The BLS key will use an empty password if the flag is set.")
	return cmd
}
