package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
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
		Use:   "register-consumer <consumer-id> <name> <description> <consumer-max-multi-staked-fps> [rollup-address]",
		Args:  cobra.MinimumNArgs(4),
		Short: "Registers a consumer",
		Long: strings.TrimSpace(
			`Registers a consumer with Babylon. The consumer-id must be unique and will be used to identify this consumer.
			The name and optional description help identify the purpose of this consumer.
			The consumer-max-multi-staked-fps specifies the maximum total number of finality providers (from all sources: Babylon + all consumers) that this consumer will accept in a BTC delegation. This is a limit imposed by the consumer to control the complexity/risk of multi-staked delegations they participate in.
			Must be at least 2 to allow for at least one Babylon FP and one consumer FP.`,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			consumerId := args[0]
			if consumerId == "" {
				return types.ErrEmptyConsumerId
			}
			name := args[1]
			if name == "" {
				return types.ErrEmptyConsumerName
			}
			description := args[2]
			if description == "" {
				return types.ErrEmptyConsumerDescription
			}
			consumerMaxMultiStakedFps, err := strconv.ParseUint(args[3], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid max-multi-staked-fps: %w", err)
			}
			if consumerMaxMultiStakedFps < 2 {
				return types.ErrInvalidMaxMultiStakedFps
			}
			rollupAddress := ""
			if len(args) > 4 {
				rollupAddress = args[4]
			}

			msg := types.MsgRegisterConsumer{
				Signer:                        clientCtx.FromAddress.String(),
				ConsumerId:                    consumerId,
				ConsumerName:                  name,
				ConsumerDescription:           description,
				ConsumerMaxMultiStakedFps:     uint32(consumerMaxMultiStakedFps),
				RollupFinalityContractAddress: rollupAddress,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
