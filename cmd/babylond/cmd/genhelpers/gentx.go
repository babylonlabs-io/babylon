// This file is derived from the Cosmos SDK repository (https://github.com/cosmos/cosmos-sdk),
// originally licensed under the Apache License, Version 2.0.

package genhelpers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	appsigner "github.com/babylonlabs-io/babylon/app/signer"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"

	"cosmossdk.io/core/address"
	sdkerrors "cosmossdk.io/errors"

	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	authclient "github.com/cosmos/cosmos-sdk/x/auth/client"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GenTxCmd builds the application's gentx command.
// Code coppied from cosmos-sdk https://github.com/cosmos/cosmos-sdk/blob/eba0b9e9c2b5da19ccb8b4b189c1f0747830a8fa/x/genutil/client/cli/gentx.go#L31
// Modified to generate MsgWrappedCreateValidator from `x/checkpointing` instead of MsgCreateValidator from `x/staking`
func GenTxCmd(mbm module.BasicManager, txEncCfg client.TxEncodingConfig, genBalIterator types.GenesisBalancesIterator, defaultNodeHome string, valAdddressCodec address.Codec) *cobra.Command {
	ipDefault, _ := server.ExternalIP()
	fsCreateValidator, defaultsDesc := cli.CreateValidatorMsgFlagSet(ipDefault)

	cmd := &cobra.Command{
		Use:   "gentx [key_name] [amount]",
		Short: "Generate a genesis tx carrying a self delegation",
		Args:  cobra.ExactArgs(2),
		Long: fmt.Sprintf(`Generate a genesis transaction that creates a validator with a self-delegation
and a BLS key if needed.
This transaction is signed by the key in the Keyring referenced by a given name. A node ID and consensus
pubkey may optionally be provided. If they are omitted, they will be retrieved from the priv_validator.json
file. The following default parameters are included:
    %s

Example:
$ %s gentx my-key-name 1000000stake --home=/path/to/home/dir --keyring-backend=os --chain-id=test-chain-1 \
    --moniker="myvalidator" \
    --commission-max-change-rate=0.01 \
    --commission-max-rate=1.0 \
    --commission-rate=0.07 \
    --details="..." \
    --security-contact="..." \
    --website="..."
`, defaultsDesc, version.AppName,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverCtx := server.GetServerContextFromCmd(cmd)
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cdc := clientCtx.Codec

			config := serverCtx.Config
			config.SetRoot(clientCtx.HomeDir)

			nodeID, valPubKey, err := genutil.InitializeNodeValidatorFiles(serverCtx.Config)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to initialize node validator files")
			}

			// read --nodeID, if empty take it from priv_validator.json
			if nodeIDString, _ := cmd.Flags().GetString(cli.FlagNodeID); nodeIDString != "" {
				nodeID = nodeIDString
			}

			// read --pubkey, if empty take it from priv_validator.json
			if pkStr, _ := cmd.Flags().GetString(cli.FlagPubKey); pkStr != "" {
				if err := clientCtx.Codec.UnmarshalInterfaceJSON([]byte(pkStr), &valPubKey); err != nil {
					return sdkerrors.Wrap(err, "failed to unmarshal validator public key")
				}
			}

			appGenesis, err := types.AppGenesisFromFile(config.GenesisFile())
			if err != nil {
				return sdkerrors.Wrapf(err, "failed to read genesis doc file %s", config.GenesisFile())
			}

			var genesisState map[string]json.RawMessage
			if err = json.Unmarshal(appGenesis.AppState, &genesisState); err != nil {
				return sdkerrors.Wrap(err, "failed to unmarshal genesis state")
			}

			if err = mbm.ValidateGenesis(cdc, txEncCfg, genesisState); err != nil {
				return sdkerrors.Wrap(err, "failed to validate genesis state")
			}

			inBuf := bufio.NewReader(cmd.InOrStdin())

			name := args[0]
			key, err := clientCtx.Keyring.Key(name)
			if err != nil {
				return sdkerrors.Wrapf(err, "failed to fetch '%s' from the keyring", name)
			}

			moniker := config.Moniker
			if m, _ := cmd.Flags().GetString(cli.FlagMoniker); m != "" {
				moniker = m
			}

			// set flags for creating a gentx
			createValCfg, err := cli.PrepareConfigForTxCreateValidator(cmd.Flags(), moniker, nodeID, appGenesis.ChainID, valPubKey)
			if err != nil {
				return sdkerrors.Wrap(err, "error creating configuration to create validator msg")
			}

			amount := args[1]
			coins, err := sdk.ParseCoinsNormalized(amount)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to parse coins")
			}
			addr, err := key.GetAddress()
			if err != nil {
				return err
			}
			err = genutil.ValidateAccountInGenesis(genesisState, genBalIterator, addr, coins, cdc)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to validate account in genesis")
			}

			txFactory, err := tx.NewFactoryCLI(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			pub, err := key.GetAddress()
			if err != nil {
				return err
			}
			clientCtx = clientCtx.WithInput(inBuf).WithFromAddress(pub)

			// The following line comes from a discrepancy between the `gentx`
			// and `create-validator` commands:
			// - `gentx` expects amount as an arg,
			// - `create-validator` expects amount as a required flag.
			// ref: https://github.com/cosmos/cosmos-sdk/issues/8251
			// Since gentx doesn't set the amount flag (which `create-validator`
			// reads from), we copy the amount arg into the valCfg directly.
			//
			// Ideally, the `create-validator` command should take a validator
			// config file instead of so many flags.
			// ref: https://github.com/cosmos/cosmos-sdk/issues/8177
			createValCfg.Amount = amount

			// create a 'create-validator' message
			txBldr, pMsgStkCreateVal, err := cli.BuildCreateValidatorMsg(clientCtx, createValCfg, txFactory, true, valAdddressCodec)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to build create-validator message")
			}

			msgStkCreateVal, ok := pMsgStkCreateVal.(*stakingtypes.MsgCreateValidator)
			if !ok {
				return sdkerrors.Wrap(err, "failed to parse msg as *stakingtypes.MsgCreateValidator message")
			}

			// bls load/create

			ck, err := appsigner.LoadConsensusKey(clientCtx.HomeDir)
			if err != nil {
				return fmt.Errorf("failed to load key from %s: %w", clientCtx.HomeDir, err)
			}

			outputFileName, err := appsigner.ExportGenBls(
				sdk.ValAddress(addr),
				ck.Comet.PrivKey,
				ck.Bls.PrivKey,
				filepath.Join(clientCtx.HomeDir, cmtconfig.DefaultConfigDir),
			)
			if err != nil {
				return err
			}

			genBls, err := checkpointingtypes.LoadGenesisKeyFromFile(outputFileName)
			if err != nil {
				return fmt.Errorf("failed to load BLS key from genesis file path %s: %w", outputFileName, err)
			}
			cmd.PrintErrf("successfully created bls PoP in %q.\n", outputFileName)

			err = genBls.Validate()
			if err != nil {
				return err
			}

			// add-bls probably not needed as the BLS keys would be included during x/checkpointing
			// msg server implementation
			msg, err := checkpointingtypes.NewMsgWrappedCreateValidator(msgStkCreateVal, genBls.BlsKey.Pubkey, genBls.BlsKey.Pop)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to build wrapped create-validator message")
			}

			if key.GetType() == keyring.TypeOffline || key.GetType() == keyring.TypeMulti {
				cmd.PrintErrln("Offline key passed in. Use `tx sign` command to sign.")
				return txBldr.PrintUnsignedTx(clientCtx, msg)
			}

			// write the unsigned transaction to the buffer
			w := bytes.NewBuffer([]byte{})
			clientCtx = clientCtx.WithOutput(w)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			if err = txBldr.PrintUnsignedTx(clientCtx, msg); err != nil {
				return sdkerrors.Wrap(err, "failed to print unsigned std tx")
			}

			// read the transaction
			stdTx, err := readUnsignedGenTxFile(clientCtx, w)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to read unsigned gen tx file")
			}

			// sign the transaction and write it to the output file
			txBuilder, err := clientCtx.TxConfig.WrapTxBuilder(stdTx)
			if err != nil {
				return fmt.Errorf("error creating tx builder: %w", err)
			}

			err = authclient.SignTx(txFactory, clientCtx, name, txBuilder, true, true)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to sign std tx")
			}

			outputDocument, _ := cmd.Flags().GetString(flags.FlagOutputDocument)
			if outputDocument == "" {
				outputDocument, err = makeOutputFilepath(config.RootDir, nodeID)
				if err != nil {
					return sdkerrors.Wrap(err, "failed to create output file path")
				}
			}

			if err := writeSignedGenTx(clientCtx, outputDocument, stdTx); err != nil {
				return sdkerrors.Wrap(err, "failed to write signed gen tx")
			}

			cmd.PrintErrf("Genesis transaction written to %q\n", outputDocument)
			return nil
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	cmd.Flags().String(flags.FlagOutputDocument, "", "Write the genesis transaction JSON document to the given file instead of the default location")
	cmd.Flags().AddFlagSet(fsCreateValidator)
	flags.AddTxFlagsToCmd(cmd)
	_ = cmd.Flags().MarkHidden(flags.FlagOutput) // signing makes sense to output only json

	return cmd
}

func PathsNodeCfg(homeDir string) (keyPath, statePath string) {
	nodeCfg := cmtconfig.DefaultConfig()
	keyPath = filepath.Join(homeDir, nodeCfg.PrivValidatorKeyFile())
	statePath = filepath.Join(homeDir, nodeCfg.PrivValidatorStateFile())
	return keyPath, statePath
}

func makeOutputFilepath(rootDir, nodeID string) (string, error) {
	writePath := filepath.Join(rootDir, "config", "gentx")
	if err := os.MkdirAll(writePath, 0o700); err != nil {
		return "", fmt.Errorf("could not create directory %q: %w", writePath, err)
	}

	return filepath.Join(writePath, fmt.Sprintf("gentx-%v.json", nodeID)), nil
}

func readUnsignedGenTxFile(clientCtx client.Context, r io.Reader) (sdk.Tx, error) {
	bz, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	aTx, err := clientCtx.TxConfig.TxJSONDecoder()(bz)
	if err != nil {
		return nil, err
	}

	return aTx, err
}

func writeSignedGenTx(clientCtx client.Context, outputDocument string, tx sdk.Tx) error {
	outputFile, err := os.OpenFile(outputDocument, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	json, err := clientCtx.TxConfig.TxJSONEncoder()(tx)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(outputFile, "%s\n", json)

	return err
}
