package cli

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
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
		NewRegisterConsumerCmd(),
	)

	return cmd
}

func NewRegisterConsumerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-consumer <consumer-id> <name> [description]",
		Args:  cobra.ExactArgs(3),
		Short: "Registers a consumer",
		Long: strings.TrimSpace(
			`Registers a consumer with Babylon. The consumer-id must be unique and will be used to identify this consumer.
			The name and optional description help identify the purpose of this consumer.`,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			consumerId := args[0]
			if consumerId == "" {
				return fmt.Errorf("consumer's id cannot be empty")
			}
			name := args[1]
			if name == "" {
				return fmt.Errorf("consumer's name cannot be empty")
			}
			description := args[2]
			if description == "" {
				return fmt.Errorf("consumer's description cannot be empty")
			}

			msg := types.MsgRegisterConsumer{
				Signer:              clientCtx.FromAddress.String(),
				ConsumerId:          consumerId,
				ConsumerName:        name,
				ConsumerDescription: description,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
