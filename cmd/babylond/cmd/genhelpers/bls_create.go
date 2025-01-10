package genhelpers

import (
	"path/filepath"
	"strings"

	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/privval"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

// CmdCreateBls CLI command to create BLS file with proof of possession.
func CmdCreateBls() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-bls",
		Short: "Create genesis BLS key file for the validator",
		Long: strings.TrimSpace(`genbls will create a BLS key file that consists of
{address, bls_pub_key, pop, pub_key} where pop is the proof-of-possession that proves
the ownership of bls_pub_key which is bonded with pub_key.

The pre-conditions of running the generate-genesis-bls-key are the existence of the keyring,
and the existence of priv_validator_key.json which contains the validator private key.


Example:
$ babylond genbls --home ./
`),

		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, _ := cmd.Flags().GetString(flags.FlagHome)

			nodeCfg := cmtconfig.DefaultConfig()
			blsCfg := privval.DefaultBlsConfig()

			keyPath := filepath.Join(homeDir, nodeCfg.PrivValidatorKeyFile())
			statePath := filepath.Join(homeDir, nodeCfg.PrivValidatorStateFile())
			blsKeyPath := filepath.Join(homeDir, blsCfg.BlsKeyFile())
			blsPasswordPath := filepath.Join(homeDir, blsCfg.BlsPasswordFile())

			if err := privval.IsValidFilePath(keyPath, statePath, blsKeyPath, blsPasswordPath); err != nil {
				return err
			}

			filePV := cmtprivval.LoadFilePV(keyPath, statePath)
			blsPV := privval.LoadBlsPV(blsKeyPath, blsPasswordPath)

			wrappedPV := &privval.WrappedFilePV{
				Key: privval.WrappedFilePVKey{
					CometPVKey: filePV.Key,
					BlsPVKey:   blsPV.Key,
				},
				LastSignState: filePV.LastSignState,
			}

			outputFileName, err := wrappedPV.ExportGenBls(filepath.Dir(keyPath))
			if err != nil {
				return err
			}

			cmd.PrintErrf("Genesis BLS keys written to %q\n", outputFileName)
			return nil
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")

	return cmd
}
