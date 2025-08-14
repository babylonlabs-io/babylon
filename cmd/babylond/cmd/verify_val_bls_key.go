package cmd

import (
	"fmt"
	"strings"

	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v4/app"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

func VerifyValidatorBlsKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify-validator-bls-key [validator-address]",
		Args:  cobra.ExactArgs(1),
		Short: "Verify the BLS key with a validator",
		Long: strings.TrimSpace(`Verify the BLS key with a validator.

The command will check if the local BLS key is associated with the specified validator address.
Password precedence for decrypting the key:
1. Environment variable BABYLON_BLS_PASSWORD
2. Password file (from --bls-password-file or default location)
3. Prompt the user for password

Example:
$ babylond verify-validator-bls-key babylonvaloper1...
$ babylond verify-validator-bls-key babylonvaloper1... --bls-password-file=/path/to/password.txt
$ babylond verify-validator-bls-key babylonvaloper1... --no-bls-password
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate validator address
			// Check address is valid before processing further
			// is more effective than checking after checking flags
			valAddrStr := args[0]
			_, err := sdk.ValAddressFromBech32(valAddrStr)
			if err != nil {
				return fmt.Errorf("invalid validator address: %w", err)
			}

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

			// Get BLS public key
			// Get BLS key information
			info, err := appsigner.ShowBlsKey(blsKeyFile, password)
			if err != nil {
				return fmt.Errorf("failed to show BLS key: %w", err)
			}

			blsPubkeyHex, ok := info["pubkey_hex"]
			if !ok {
				return fmt.Errorf("failed to get BLS public key from info")
			}

			// Set client context for checkpointing module
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return fmt.Errorf("failed to get client context: %w", err)
			}

			// Get query client context for checkpointing module and epoching module
			checkpointingQueryClient := checkpointingtypes.NewQueryClient(clientCtx)
			epochingQueryClient := epochingtypes.NewQueryClient(clientCtx)

			// Get epoch response
			epochResponse, err := epochingQueryClient.CurrentEpoch(
				cmd.Context(),
				&epochingtypes.QueryCurrentEpochRequest{},
			)
			if err != nil {
				return fmt.Errorf("failed to query current epoch: %w", err)
			}

			// Get BLS public key list at current epoch
			resp, err := checkpointingQueryClient.BlsPublicKeyList(
				cmd.Context(),
				&checkpointingtypes.QueryBlsPublicKeyListRequest{
					EpochNum: epochResponse.CurrentEpoch,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to query validator address for BLS key: %w", err)
			}

			// Check validator is valid with bls key
			for _, r := range resp.GetValidatorWithBlsKeys() {
				if r.GetValidatorAddress() == valAddrStr && r.GetBlsPubKeyHex() == blsPubkeyHex {
					cmd.Println(fmt.Sprintf("Verification SUCCESSFUL: The BLS key is correctly associated with validator %s\n", valAddrStr))
					return nil
				}
			}
			return fmt.Errorf("BLS key is not associated with the specified validator")
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "Use empty password for BLS key")
	cmd.Flags().String(flagBlsPasswordFile, "", "Path to a file containing the BLS password")

	return cmd
}
