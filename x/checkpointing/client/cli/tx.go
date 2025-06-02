package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cosmossdk.io/core/address"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	cosmoscli "github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	"github.com/cosmos/cosmos-sdk/client"
)

const (
	FlagBlsPopFilePath = "bls-pop"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdWrappedCreateValidator(authcodec.NewBech32Codec(appparams.Bech32PrefixValAddr)))

	return cmd
}

func CmdWrappedCreateValidator(valAddrCodec address.Codec) *cobra.Command {
	cmd := cosmoscli.NewCreateValidatorCmd(valAddrCodec)
	cmd.Long = strings.TrimSpace(`create-validator will create a new validator identified by both Ed25519 key and BLS key.
This command creates a MsgWrappedCreateValidator message which is a wrapper of Cosmos SDK's
MsgCreateValidator with a pair of BLS key.

If --bls-pop is specified, it will use the BLS pop generated via 'babylond generate-bls-pop'.
If not specified, the BLS pop will be generated from priv_validator_key.json and bls_key.json.`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		clientCtx, err := client.GetClientTxContext(cmd)
		if err != nil {
			return fmt.Errorf("failed to get client tx context: %w", err)
		}

		txf, err := tx.NewFactoryCLI(clientCtx, cmd.Flags())
		if err != nil {
			return fmt.Errorf("failed to create transaction factory: %w", err)
		}

		val, err := parseAndValidateValidatorJSON(clientCtx.Codec, args[0])
		if err != nil {
			return fmt.Errorf("failed to parse and validate validator JSON: %w", err)
		}

		txf = txf.WithTxConfig(clientCtx.TxConfig).WithAccountRetriever(clientCtx.AccountRetriever)

		txf, msg, err := buildWrappedCreateValidatorMsg(clientCtx, txf, cmd.Flags(), val, valAddrCodec)
		if err != nil {
			return fmt.Errorf("failed to build wrapped create validator message: %w", err)
		}

		return tx.GenerateOrBroadcastTxWithFactory(clientCtx, txf, msg)
	}
	// HACK: test cases need to setup the path where the priv validator BLS key is going to be set
	// so we redefine the FlagHome here. Since we can't import `app` due to a cyclic dependency,
	// we have to duplicate the definition here.
	// If this changes, the `DefaultHomeDir` flag at `app/app.go` needs to change as well.
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	defaultNodeHome := filepath.Join(userHomeDir, ".babylond")
	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The node home directory")
	cmd.Flags().String(FlagBlsPopFilePath, "", "The path to the BLS proof-of-possession file generated via `babylon generate-bls-pop`")

	return cmd
}
