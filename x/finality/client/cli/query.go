package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/x/finality/types"
)

const (
	flagQueriedBlockStatus = "queried-block-status"
	flagStartHeight        = "start-height"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd(queryRoute string) *cobra.Command {
	// Group finality queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdQueryParams())
	cmd.AddCommand(CmdListPublicRandomness())
	cmd.AddCommand(CmdListPubRandCommit())
	cmd.AddCommand(CmdBlock())
	cmd.AddCommand(CmdListBlocks())
	cmd.AddCommand(CmdVotesAtHeight())
	cmd.AddCommand(CmdListEvidences())
	cmd.AddCommand(CmdSigningInfo())
	cmd.AddCommand(CmdAllSigningInfo())

	return cmd
}

func CmdVotesAtHeight() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "votes-at-height [height]",
		Short: "retrieve all finality provider pks who voted at requested babylon height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			height, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			res, err := queryClient.VotesAtHeight(cmd.Context(), &types.QueryVotesAtHeightRequest{Height: height})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdListPublicRandomness() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-public-randomness [fp_btc_pk_hex]",
		Short: "list public randomness committed by a given finality provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.ListPublicRandomness(cmd.Context(), &types.QueryListPublicRandomnessRequest{
				FpBtcPkHex: args[0],
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "list-public-randomness")

	return cmd
}

func CmdListPubRandCommit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-pub-rand-commit [fp_btc_pk_hex]",
		Short: "list public randomness commitment of a given finality provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.ListPubRandCommit(cmd.Context(), &types.QueryListPubRandCommitRequest{
				FpBtcPkHex: args[0],
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "list-pub-rand-commit")

	return cmd
}

func CmdListBlocks() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-blocks",
		Short: "list blocks at a given status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}
			queriedBlockStatusString, err := cmd.Flags().GetString(flagQueriedBlockStatus)
			if err != nil {
				return err
			}
			queriedBlockStatus, err := types.NewQueriedBlockStatus(queriedBlockStatusString)
			if err != nil {
				return err
			}

			res, err := queryClient.ListBlocks(cmd.Context(), &types.QueryListBlocksRequest{
				Status:     queriedBlockStatus,
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "list-blocks")
	cmd.Flags().String(flagQueriedBlockStatus, "Any", "Status of the queried blocks (NonFinalized|Finalized|Any)")

	return cmd
}

func CmdBlock() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block [height]",
		Short: "show the information of the block at a given height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			queriedBlockHeight, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			res, err := queryClient.Block(cmd.Context(), &types.QueryBlockRequest{
				Height: queriedBlockHeight,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdListEvidences() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-evidences",
		Short: "list equivocation evidences since a given height",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}
			startHeight, err := cmd.Flags().GetUint64(flagStartHeight)
			if err != nil {
				return err
			}

			res, err := queryClient.ListEvidences(cmd.Context(), &types.QueryListEvidencesRequest{
				StartHeight: startHeight,
				Pagination:  pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "list-evidences")
	cmd.Flags().Uint64(flagStartHeight, 0, "Starting height for scanning evidences")

	return cmd
}

func CmdSigningInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signing-info [fp-pk-hex]",
		Short: "Show signing info of a given finality provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			fpPkHex := args[0]

			// query for the signing info of a given finality provider
			res, err := queryClient.SigningInfo(
				cmd.Context(),
				&types.QuerySigningInfoRequest{FpBtcPkHex: fpPkHex},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdAllSigningInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all-signing-info",
		Short: "Show signing info of finality providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			// query for all the signing infos
			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.SigningInfos(
				cmd.Context(),
				&types.QuerySigningInfosRequest{Pagination: pageReq},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "all-signing-info")

	return cmd
}
