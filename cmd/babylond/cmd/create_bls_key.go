package cmd

import (
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/privval"
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
			password, _ := cmd.Flags().GetString(flagBlsPassword)
			return CreateBlsKey(homeDir, password)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().String(flagBlsPassword, "", "The password for the BLS key. If a flag is set, the non-empty password should be provided. If a flag is not set, the password will be read from the prompt.")
	return cmd
}

func CreateBlsKey(homeDir, password string) error {
	if password == "" {
		password = privval.NewBlsPassword()
	}

	privval.GenBlsPV(
		privval.DefaultBlsKeyFile(homeDir),
		privval.DefaultBlsPasswordFile(homeDir),
		password,
	)
	return nil
}
