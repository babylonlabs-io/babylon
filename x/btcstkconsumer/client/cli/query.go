package cli

import (
	"fmt"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
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
	cmd.AddCommand(CmdRegisteredChain())
	cmd.AddCommand(CmdRegisteredChains())
	cmd.AddCommand(CmdFinalityProviderChain())
	cmd.AddCommand(CmdFinalityProvider())
	cmd.AddCommand(CmdFinalityProviders())

	return cmd
}

func CmdRegisteredChains() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registered-chains",
		Short: "retrieve list of registered chains",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.ChainRegistryList(cmd.Context(), &types.QueryChainRegistryListRequest{
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "registered-chains")

	return cmd
}

func CmdRegisteredChain() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registered-chain <chain-id>",
		Short: "retrieve a given registered chain's info",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ChainsRegistry(
				cmd.Context(),
				&types.QueryChainsRegistryRequest{
					ChainIds: []string{args[0]},
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

func CmdFinalityProviderChain() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-provider-chain <fp_btc_pk_hex>",
		Short: "retrieve a given CZ finality provider's registered chain id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FinalityProviderChain(
				cmd.Context(),
				&types.QueryFinalityProviderChainRequest{
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

func CmdFinalityProvider() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-provider <chain-id> <fp_btc_pk_hex>",
		Short: "retrieve a given chain's finality provider",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FinalityProvider(
				cmd.Context(),
				&types.QueryFinalityProviderRequest{
					FpBtcPkHex: args[1],
					ChainId:    args[0],
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
		Use:   "finality-providers <chain-id>",
		Short: "retrieve a given chain's all finality providers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.FinalityProviders(cmd.Context(), &types.QueryFinalityProvidersRequest{
				ChainId:    args[0],
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
