package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
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

	cmd.AddCommand(
		CmdQueryParams(),
		CmdFinalityProvidersAtHeight(),
		CmdFinalityProviderPowerAtHeight(),
		CmdActivatedHeight(),
		CmdListPublicRandomness(),
		CmdListPubRandCommit(),
		CmdBlock(),
		CmdListBlocks(),
		CmdVotesAtHeight(),
		CmdVotingPowerDistribution(),
		CmdListEvidences(),
		CmdSigningInfo(),
		CmdAllSigningInfo(),
	)

	return cmd
}

func CmdFinalityProviderPowerAtHeight() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-provider-power-at-height [fp_btc_pk_hex] [height]",
		Short: "get the voting power of a given finality provider at a given height",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			height, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}
			res, err := queryClient.FinalityProviderPowerAtHeight(cmd.Context(), &types.QueryFinalityProviderPowerAtHeightRequest{
				FpBtcPkHex: args[0],
				Height:     height,
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

func CmdActivatedHeight() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activated-height",
		Short: "get activated height, i.e., the first height where there exists 1 finality provider with voting power",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.ActivatedHeight(cmd.Context(), &types.QueryActivatedHeightRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdFinalityProvidersAtHeight() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-providers-at-height [height]",
		Short: "retrieve all finality providers at a given babylon height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			height, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.ActiveFinalityProvidersAtHeight(cmd.Context(), &types.QueryActiveFinalityProvidersAtHeightRequest{
				Height:     height,
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "finality-providers-at-height")

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

func CmdVotingPowerDistribution() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vp-dst-cache [height]",
		Short: "retrieve the voting power distribution at a given height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			height, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			res, err := queryClient.VotingPowerDistribution(cmd.Context(), &types.QueryVotingPowerDistributionRequest{Height: height})
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
