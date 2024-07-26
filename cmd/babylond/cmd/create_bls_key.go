package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/privval"
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

			return CreateBlsKey(homeDir, addr)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")

	return cmd
}

func CreateBlsKey(home string, addr sdk.AccAddress) error {
	nodeCfg := cmtconfig.DefaultConfig()
	keyPath := filepath.Join(home, nodeCfg.PrivValidatorKeyFile())
	statePath := filepath.Join(home, nodeCfg.PrivValidatorStateFile())

	pv, err := LoadWrappedFilePV(keyPath, statePath)
	if err != nil {
		return err
	}

	wrappedPV := privval.NewWrappedFilePV(pv.GetValPrivKey(), bls12381.GenPrivKey(), keyPath, statePath)
	wrappedPV.SetAccAddress(addr)

	return nil
}

// LoadWrappedFilePV loads the wrapped file private key from the file path.
func LoadWrappedFilePV(keyPath, statePath string) (*privval.WrappedFilePV, error) {
	if !cmtos.FileExists(keyPath) {
		return nil, errors.New("validator key file does not exist")
	}
	if !cmtos.FileExists(statePath) {
		return nil, errors.New("validator state file does not exist")
	}
	return privval.LoadWrappedFilePV(keyPath, statePath), nil
}
