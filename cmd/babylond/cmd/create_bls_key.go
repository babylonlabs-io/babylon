package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	"github.com/babylonlabs-io/babylon/privval"
	cmtprivval "github.com/cometbft/cometbft/privval"
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
	cmtPv := cmtprivval.LoadFilePV(keyPath, statePath)

	blsCfg := privval.DefaultBlsConfig()
	blsKeyPath := filepath.Join(home, blsCfg.BlsKeyFile())
	blsPasswordFile := filepath.Join(home, blsCfg.BlsPasswordFile())

	var blsPassword string
	var err error
	if !cmtos.FileExists(blsPasswordFile) {
		log.Printf("BLS password file don't exists in file: %v", blsPasswordFile)
		inBuf := bufio.NewReader(os.Stdin)
		blsPassword, err = input.GetString("Enter your bls password", inBuf)
		if err != nil {
			return err
		}
		err = erc2335.SavePasswordToFile(blsPassword, blsPasswordFile)
		if err != nil {
			return err
		}
	}
	blsPv := privval.NewBlsPV(bls12381.GenPrivKey(), blsKeyPath, blsPasswordFile)
	blsPv.Key.Save(blsPassword)

	wrappedPv := privval.WrappedFilePV{
		Key: privval.WrappedFilePVKey{
			CometPVKey: cmtPv.Key,
			BlsPVKey:   blsPv.Key,
		},
		LastSignState: cmtPv.LastSignState,
	}

	wrappedPv.SetAccAddress(addr)
	log.Printf("Saved delegator address: %s", addr.String())
	log.Printf("Saved delegator address in wrapperPv: %s", wrappedPv.Key.DelegatorAddress)
	return nil
}
