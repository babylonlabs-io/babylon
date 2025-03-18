package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
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

			if noBlsPassword {
				blsKeyFile := appsigner.DefaultBlsKeyFile(homeDir)
				blsPasswordFile := appsigner.DefaultBlsPasswordFile(homeDir)

				if err := appsigner.EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
					return fmt.Errorf("failed to ensure dirs exist: %w", err)
				}

				bls := appsigner.NewBls(bls12381.GenPrivKey(), blsKeyFile, blsPasswordFile)
				bls.Key.Save("")
				fmt.Printf("BLS key generated successfully without password protection.\n")
				return nil
			}

			fmt.Println("\nSelect the storage strategy for your BLS password.")
			fmt.Println("1. Environment variable (recommended)")
			fmt.Println("2. Local file (not recommended)")

			choice, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			choice = strings.TrimSpace(choice)

			password, _ := cmd.Flags().GetString(flagBlsPassword)
			if password == "" {
				fmt.Println("\nNow, please enter a secure password for your BLS key.")
				fmt.Println("This password will be used to encrypt your validator's BLS key.")
				password, err = input.GetString("Enter your BLS password", bufio.NewReader(os.Stdin))
				if err != nil {
					return fmt.Errorf("failed to get BLS password: %w", err)
				}
			}

			blsKeyFile := appsigner.DefaultBlsKeyFile(homeDir)

			if choice == "1" {
				if err := appsigner.EnsureDirs(blsKeyFile); err != nil {
					return fmt.Errorf("failed to ensure dirs exist: %w", err)
				}

				err := os.Setenv(appsigner.BlsPasswordEnvVar, password)
				if err != nil {
					return fmt.Errorf("failed to set password in environment")
				}

				bls := appsigner.NewBls(bls12381.GenPrivKey(), blsKeyFile, "")
				bls.Key.Save(password)

				fmt.Printf("\nIMPORTANT: Your BLS key has been created with password protection.\n")
				fmt.Printf("You must set the BABYLON_BLS_PASSWORD environment variable before starting the node:\n")
				fmt.Printf("export %s=<your_password>\n", appsigner.BlsPasswordEnvVar)
				return nil
			}

			if choice == "2" {
				var passwordFile string
				fmt.Println("\nWhere would you like to save your password file?")
				fmt.Println("1. Default location")
				fmt.Println("2. Custom location")

				fileChoice, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				fileChoice = strings.TrimSpace(fileChoice)

				if fileChoice == "1" {
					passwordFile = appsigner.DefaultBlsPasswordFile(homeDir)
					fmt.Println("Your password will be saved to the default location.")
				} else {
					fmt.Println("Please enter the absolute path where you want to save your password file:")
					fmt.Println("(If you provide a directory path, the file will be named bls_password.txt)")
					customPath, err := bufio.NewReader(os.Stdin).ReadString('\n')
					if err != nil {
						return fmt.Errorf("failed to read input: %w", err)
					}
					passwordFile = strings.TrimSpace(customPath)

					fileInfo, err := os.Stat(passwordFile)
					if err == nil && fileInfo.IsDir() {
						passwordFile = filepath.Join(passwordFile, appsigner.DefaultBlsPasswordName)
					}
				}

				if err := appsigner.EnsureDirs(blsKeyFile, passwordFile); err != nil {
					return fmt.Errorf("failed to ensure dirs exist: %w", err)
				}

				bls := appsigner.NewBls(bls12381.GenPrivKey(), blsKeyFile, passwordFile)
				bls.Key.Save(password)

				fmt.Printf("\nIMPORTANT: Your BLS key has been created with password protection.\n")
				fmt.Println("Your password has been saved to the specified location.")
				fmt.Println("You will need this file when starting your node.")
				return nil
			}

			return fmt.Errorf("invalid choice: %s", choice)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().String(flagBlsPassword, "", "The password for the BLS key. If the flag is not set, the password will be read from the prompt.")
	cmd.Flags().Bool(flagNoBlsPassword, false, "The BLS key will use an empty password if the flag is set.")
	return cmd
}

// blsPassword returns the password for the BLS key.
// If the noBlsPassword flag is set, the function returns an empty string.
// If the blsPassword flag is set but no argument, the function returns "flag needs an argument: --bls-password" error.
// If the blsPassword flag is set with non-empty string, the function returns the value of the flag.
// If the blsPassword flag is set with empty string, the function requires the user to enter a password.
// If the blsPassword flag is not set and the noBlsPassword flag is not set, the function requires the user to enter a password.
func blsPassword(cmd *cobra.Command) string {
	noBlsPassword, _ := cmd.Flags().GetBool(flagNoBlsPassword)
	if noBlsPassword {
		return ""
	}
	password, _ := cmd.Flags().GetString(flagBlsPassword)
	if password == "" {
		return appsigner.NewBlsPassword()
	}
	return password
}
