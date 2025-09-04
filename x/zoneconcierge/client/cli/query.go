package cli

import (
	"fmt"
	"strconv"

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

	cmd.AddCommand(CmdFinalizedBSNsInfo())
	cmd.AddCommand(CmdLatestEpochHeader())
	cmd.AddCommand(CmdBSNLastSentSegment())
	cmd.AddCommand(CmdQueryGetSealedEpochProof())

	return cmd
}

func CmdFinalizedBSNsInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finalized-bsns-info <bsn-ids>",
		Short: "retrieve the finalized info for a given list of BSNs",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			prove, _ := cmd.Flags().GetBool("prove")

			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			req := types.QueryFinalizedBSNsInfoRequest{ConsumerIds: args, Prove: prove}
			resp, err := queryClient.FinalizedBSNsInfo(cmd.Context(), &req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	cmd.Flags().Bool("prove", false, "whether to retrieve proofs for each FinalizedBSNData")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdLatestEpochHeader() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "latest-epoch-header <bsn-id>",
		Short: "retrieve the latest epoch header for a given bsn",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			req := types.QueryLatestEpochHeaderRequest{ConsumerId: args[0]}
			resp, err := queryClient.LatestEpochHeader(cmd.Context(), &req)
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdBSNLastSentSegment() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bsn-last-sent-seg <bsn-id>",
		Short: "retrieve the last sent segment of a given bsn",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			req := types.QueryBSNLastSentSegmentRequest{ConsumerId: args[0]}
			resp, err := queryClient.BSNLastSentSegment(cmd.Context(), &req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdQueryGetSealedEpochProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-sealed-epoch-proof <epoch-num>",
		Short: "retrieve the sealed epoch proof of a given epoch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			req := types.QueryGetSealedEpochProofRequest{EpochNum: epoch}
			resp, err := queryClient.GetSealedEpochProof(cmd.Context(), &req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
