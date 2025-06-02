package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd(queryRoute string) *cobra.Command {
	// Group btcstaking queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdQueryParams())
	cmd.AddCommand(CmdQueryParamsVersions())
	cmd.AddCommand(CmdFinalityProvider())
	cmd.AddCommand(CmdFinalityProviders())
	cmd.AddCommand(CmdBTCDelegations())
	cmd.AddCommand(CmdFinalityProviderDelegations())
	cmd.AddCommand(CmdDelegation())
	cmd.AddCommand(CmdQueryParamsByVersion())
	cmd.AddCommand(CmdQueryLargestBtcReOrg())

	return cmd
}

func CmdFinalityProvider() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-provider [fp_btc_pk_hex]",
		Short: "retrieve a finality provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FinalityProvider(
				cmd.Context(),
				&types.QueryFinalityProviderRequest{
					FpBtcPkHex: args[0],
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

func CmdDelegation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delegation [staking_tx_hash_hex]",
		Short: "retrieve a BTC delegation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.BTCDelegation(
				cmd.Context(),
				&types.QueryBTCDelegationRequest{
					StakingTxHashHex: args[0],
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

func CmdFinalityProviders() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-providers",
		Short: "retrieve all finality providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.FinalityProviders(cmd.Context(), &types.QueryFinalityProvidersRequest{
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "finality-providers")

	return cmd
}

func CmdBTCDelegations() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "btc-delegations [status]",
		Short: "retrieve all BTC delegations under the given status (pending, active, unbonding, unbonded, any)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			status, err := types.NewBTCDelegationStatusFromString(args[0])
			if err != nil {
				return err
			}

			res, err := queryClient.BTCDelegations(cmd.Context(), &types.QueryBTCDelegationsRequest{
				Status:     status,
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "btc-delegations")

	return cmd
}

func CmdFinalityProviderDelegations() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-provider-delegations [fp_pk_hex]",
		Short: "retrieve all delegations under a given finality provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.FinalityProviderDelegations(cmd.Context(), &types.QueryFinalityProviderDelegationsRequest{
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
	flags.AddPaginationFlagsToCmd(cmd, "finality-provider-delegations")

	return cmd
}

// CmdQueryParamsByVersion implements the query params by version command.
func CmdQueryParamsByVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params-by-version [version]",
		Short: "Query BTC staking parameters by version number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			version, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("version must be a positive 32-bit integer: %w", err)
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ParamsByVersion(
				context.Background(),
				&types.QueryParamsByVersionRequest{Version: uint32(version)},
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

// CmdQueryLargestBtcReOrg implements the query of the largest BTC reorganization recorded command.
func CmdQueryLargestBtcReOrg() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "largest-btc-reorg",
		Short: "Query the larges BTC reorganization recorded",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.LargestBtcReOrg(
				context.Background(),
				&types.QueryLargestBtcReOrgRequest{},
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
