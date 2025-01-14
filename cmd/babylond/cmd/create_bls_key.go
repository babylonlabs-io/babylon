package cmd

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/privval"
)

const (
	FlagPassword = "bls-password"
)

func CreateBlsKeyCmd() *cobra.Command {
	bech32PrefixAccAddr := appparams.Bech32PrefixAccAddr

	cmd := &cobra.Command{
		Use:   "create-bls-key [account-address]",
		Args:  cobra.ExactArgs(1),
		Short: "Create a pair of BLS keys for a validator",
		Long: strings.TrimSpace(
			fmt.Sprintf(`create-bls will create a pair of BLS keys that are used to
send BLS signatures for checkpointing.

BLS keys are stored along with other validator keys in priv_validator_key.json,
which should exist before running the command (via babylond init or babylond testnet).

Example:
$ babylond create-bls-key %s1f5tnl46mk4dfp4nx3n2vnrvyw2h2ydz6ykhk3r --home ./
`,
				bech32PrefixAccAddr,
			),
		),

		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, _ := cmd.Flags().GetString(flags.FlagHome)

			addr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			var password string
			password, _ = cmd.Flags().GetString(FlagPassword)
			if password == "" {
				password = privval.NewBlsPassword()
			}
			return CreateBlsKey(homeDir, password, addr)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().String(FlagPassword, "", "The password for the BLS key. If a flag is set, the non-empty password should be provided. If a flag is not set, the password will be read from the prompt.")
	return cmd
}

func CreateBlsKey(home, password string, addr sdk.AccAddress) error {
	privval.GenBlsPV(
		privval.DefaultBlsKeyFile(home),
		privval.DefaultBlsPasswordFile(home),
		password,
		addr.String(),
	)
	return nil
}
