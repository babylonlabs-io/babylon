package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/math/unsafe"
	"github.com/babylonlabs-io/babylon/privval"
	"github.com/spf13/cobra"

	errorsmod "cosmossdk.io/errors"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	cmtcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/go-bip39"
)

// wonjoon
// copied from "https://github.com/cosmos/cosmos-sdk/x/genutil/client/cli/init.go"
func InitCmd(mbm module.BasicManager, defaultNodeHome string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [moniker]",
		Short: "Initialize private validator, p2p, genesis, and application configuration files",
		Long:  `Initialize validators's and node's configuration files.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			cdc := clientCtx.Codec

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config
			config.SetRoot(clientCtx.HomeDir)

			chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
			switch {
			case chainID != "":
			case clientCtx.ChainID != "":
				chainID = clientCtx.ChainID
			default:
				chainID = fmt.Sprintf("test-chain-%v", unsafe.Str(6))
			}

			// Get bip39 mnemonic
			var mnemonic string
			recover, _ := cmd.Flags().GetBool(cmtcli.FlagRecover)
			if recover {
				inBuf := bufio.NewReader(cmd.InOrStdin())
				value, err := input.GetString("Enter your bip39 mnemonic", inBuf)
				if err != nil {
					return err
				}

				mnemonic = value
				if !bip39.IsMnemonicValid(mnemonic) {
					return errors.New("invalid mnemonic")
				}
			}

			// Get initial height
			initHeight, _ := cmd.Flags().GetInt64(flags.FlagInitHeight)
			if initHeight < 1 {
				initHeight = 1
			}

			// Get bls password
			inBuf := bufio.NewReader(cmd.InOrStdin())
			blsPassword, err := input.GetString("Enter your bls password", inBuf)
			if err != nil {
				return err
			}

			// Initialize bls key
			blsCfg := privval.BlsConfig{
				RootDir:         config.RootDir,
				BlsKeyPath:      filepath.Join(config.RootDir, cfg.DefaultConfigDir, privval.DefaultBlsKeyName),
				BlsPasswordPath: filepath.Join(config.RootDir, cfg.DefaultConfigDir, privval.DefaultBlsPasswordName),
			}

			// Initialize BLS key and save to file
			_, err = privval.InitializeBlsFileFromMnemonic(&blsCfg, blsPassword, mnemonic)
			if err != nil {
				return err
			}

			// InitializeNodeValidatorFilesFromMnemonic from cosmos-sdk/x/genutil/utils.go
			nodeID, _, err := genutil.InitializeNodeValidatorFilesFromMnemonic(config, mnemonic)
			if err != nil {
				return err
			}

			config.Moniker = args[0]

			genFile := config.GenesisFile()
			overwrite, _ := cmd.Flags().GetBool(cmtcli.FlagOverwrite)
			defaultDenom, _ := cmd.Flags().GetString(cmtcli.FlagDefaultBondDenom)

			// use os.Stat to check if the file exists
			_, err = os.Stat(genFile)
			if !overwrite && !os.IsNotExist(err) {
				return fmt.Errorf("genesis.json file already exists: %v", genFile)
			}

			// Overwrites the SDK default denom for side-effects
			if defaultDenom != "" {
				sdk.DefaultBondDenom = defaultDenom
			}
			appGenState := mbm.DefaultGenesis(cdc)

			appState, err := json.MarshalIndent(appGenState, "", " ")
			if err != nil {
				return errorsmod.Wrap(err, "Failed to marshal default genesis state")
			}

			appGenesis := &types.AppGenesis{}
			if _, err := os.Stat(genFile); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else {
				appGenesis, err = types.AppGenesisFromFile(genFile)
				if err != nil {
					return errorsmod.Wrap(err, "Failed to read genesis doc from file")
				}
			}

			appGenesis.AppName = version.AppName
			appGenesis.AppVersion = version.Version
			appGenesis.ChainID = chainID
			appGenesis.AppState = appState
			appGenesis.InitialHeight = initHeight
			appGenesis.Consensus = &types.ConsensusGenesis{
				Validators: nil,
			}

			if err = genutil.ExportGenesisFile(appGenesis, genFile); err != nil {
				return errorsmod.Wrap(err, "Failed to export genesis file")
			}

			toPrint := newPrintInfo(config.Moniker, chainID, nodeID, "", appState)

			cfg.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
			return displayInfo(toPrint)
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "node's home directory")
	cmd.Flags().BoolP(cmtcli.FlagOverwrite, "o", false, "overwrite the genesis.json file")
	cmd.Flags().Bool(cmtcli.FlagRecover, false, "provide seed phrase to recover existing key instead of creating")
	cmd.Flags().String(flags.FlagChainID, "", "genesis file chain-id, if left blank will be randomly created")
	cmd.Flags().String(cmtcli.FlagDefaultBondDenom, "", "genesis file default denomination, if left blank default value is 'stake'")
	cmd.Flags().Int64(flags.FlagInitHeight, 1, "specify the initial block height at genesis")
	cmd.Flags().String("bls-password", "", "BLS key password (if not provided, will prompt for input)")

	return cmd
}

// copied from "https://github.com/cosmos/cosmos-sdk/x/genutil/client/cli/init.go"
type printInfo struct {
	Moniker    string          `json:"moniker" yaml:"moniker"`
	ChainID    string          `json:"chain_id" yaml:"chain_id"`
	NodeID     string          `json:"node_id" yaml:"node_id"`
	GenTxsDir  string          `json:"gentxs_dir" yaml:"gentxs_dir"`
	AppMessage json.RawMessage `json:"app_message" yaml:"app_message"`
}

// copied from "https://github.com/cosmos/cosmos-sdk/x/genutil/client/cli/init.go"
func newPrintInfo(moniker, chainID, nodeID, genTxsDir string, appMessage json.RawMessage) printInfo {
	return printInfo{
		Moniker:    moniker,
		ChainID:    chainID,
		NodeID:     nodeID,
		GenTxsDir:  genTxsDir,
		AppMessage: appMessage,
	}
}

// copied from "https://github.com/cosmos/cosmos-sdk/x/genutil/client/cli/init.go"
func displayInfo(info printInfo) error {
	out, err := json.MarshalIndent(info, "", " ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(os.Stderr, "%s\n", out)

	return err
}
