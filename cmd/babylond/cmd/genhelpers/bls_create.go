package genhelpers

import (
	"fmt"
	"path/filepath"
	"strings"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
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
			nodeCfg.SetRoot(homeDir)
			cmtPvKeyFile := nodeCfg.PrivValidatorKeyFile()
			cmtPvStateFile := nodeCfg.PrivValidatorStateFile()
			blsKeyFile := privval.DefaultBlsKeyFile(homeDir)
			blsPasswordFile := privval.DefaultBlsPasswordFile(homeDir)

			if err := func(paths ...string) error {
				for _, path := range paths {
					if !cmtos.FileExists(path) {
						return fmt.Errorf("file does not exist in %s", path)
					}
				}
				return nil
			}(cmtPvKeyFile, cmtPvStateFile, blsKeyFile, blsPasswordFile); err != nil {
				return err
			}

			cmtPV := cmtprivval.LoadFilePV(cmtPvKeyFile, cmtPvStateFile)
			blsPV := privval.LoadBlsPV(blsKeyFile, blsPasswordFile)

			wrappedPV := &privval.WrappedFilePV{
				Key: privval.WrappedFilePVKey{
					CometPVKey: cmtPV.Key,
					BlsPVKey:   blsPV.Key,
				},
				LastSignState: cmtPV.LastSignState,
			}

			outputFileName, err := wrappedPV.ExportGenBls(filepath.Dir(cmtPvKeyFile))
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
