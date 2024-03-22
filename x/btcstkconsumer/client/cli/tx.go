package cli

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
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

	cmd.AddCommand(
		NewRegisterChainCmd(),
	)

	return cmd
}

func NewRegisterChainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-chain <chain-id> <name> [description]",
		Args:  cobra.RangeArgs(2, 3),
		Short: "Registers a CZ chain",
		Long: strings.TrimSpace(
			`Registers a CZ chain.`,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// get description
			chainId := args[0]
			if chainId == "" {
				return fmt.Errorf("chain's id cannot be empty")
			}
			name := args[1]
			if name == "" {
				return fmt.Errorf("chain's name cannot be empty")
			}
			description := ""
			if len(args) == 3 {
				description = args[2]
			}

			msg := types.MsgRegisterChain{
				Signer:           clientCtx.FromAddress.String(),
				ChainId:          chainId,
				ChainName:        name,
				ChainDescription: description,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
