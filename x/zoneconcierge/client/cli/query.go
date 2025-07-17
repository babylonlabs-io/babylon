package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd(queryRoute string) *cobra.Command {
	// Group zoneconcierge queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdFinalizedConsumersInfo())
	return cmd
}

func CmdFinalizedConsumersInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finalized-consumers-info <consumer-ids>",
		Short: "retrieve the finalized info for a given list of consumers",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			prove, _ := cmd.Flags().GetBool("prove")

			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			req := types.QueryFinalizedConsumersInfoRequest{ConsumerIds: args, Prove: prove}
			resp, err := queryClient.FinalizedConsumersInfo(cmd.Context(), &req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	cmd.Flags().Bool("prove", false, "whether to retrieve proofs for each FinalizedConsumerData")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
