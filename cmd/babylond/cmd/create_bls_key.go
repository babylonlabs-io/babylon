package cmd

import (
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	appsigner "github.com/babylonlabs-io/babylon/app/signer"
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
			appsigner.GenBls(appsigner.DefaultBlsKeyFile(homeDir), appsigner.DefaultBlsPasswordFile(homeDir), blsPassword(cmd))
			return nil
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
