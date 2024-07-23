package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/babylonchain/babylon/x/epoching/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd(queryRoute string) *cobra.Command {
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryEpochInfo(),
		CmdQueryEpochsInfo(),
		CmdQueryEpochMsgs(),
		CmdQueryEpochValidators(),
	)

	return cmd
}

func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "shows the parameters of the module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdQueryEpochInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch [epoch_number]",
		Short: "shows the information of the current epoch, or the given epoch if specified",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			epochNum, err := getEpoch(queryClient, args)
			if err != nil {
				return err
			}

			res, err := queryClient.EpochInfo(
				context.Background(),
				&types.QueryEpochInfoRequest{
					EpochNum: epochNum,
				},
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

func CmdQueryEpochsInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epochs",
		Short: "shows the information of epochs according to the pagination parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.EpochsInfo(
				context.Background(),
				&types.QueryEpochsInfoRequest{
					Pagination: pageReq,
				},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "epochs")

	return cmd
}

func CmdQueryEpochMsgs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch-msgs [epoch_number]",
		Short: "shows the messages that will be executed at the end of the current epoch, or the given epoch if specified",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			epochNum, err := getEpoch(queryClient, args)
			if err != nil {
				return err
			}

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.EpochMsgs(
				context.Background(),
				&types.QueryEpochMsgsRequest{
					EpochNum:   epochNum,
					Pagination: pageReq,
				},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "epoch-msgs")

	return cmd
}

func CmdQueryEpochValidators() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch-validators [epoch_number]",
		Short: "shows the validators of the current epoch, or the given epoch if specified",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			epochNum, err := getEpoch(queryClient, args)
			if err != nil {
				return err
			}

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.EpochValSet(
				context.Background(),
				&types.QueryEpochValSetRequest{
					EpochNum:   epochNum,
					Pagination: pageReq,
				},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "epoch-validators")

	return cmd
}

func getEpoch(queryClient types.QueryClient, args []string) (uint64, error) {
	var (
		epochNum uint64
		err      error
	)

	if len(args) == 0 {
		// get the current epoch number
		res, err := queryClient.CurrentEpoch(
			context.Background(),
			&types.QueryCurrentEpochRequest{},
		)
		if err != nil {
			return 0, err
		}
		epochNum = res.CurrentEpoch
	} else {
		// get the given epoch number
		epochNum, err = strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return 0, err
		}
	}

	return epochNum, nil
}
