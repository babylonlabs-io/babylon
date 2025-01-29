package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
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
	cmd.AddCommand(CmdRegisteredConsumer())
	cmd.AddCommand(CmdRegisteredConsumers())
	cmd.AddCommand(CmdFinalityProviderConsumer())
	cmd.AddCommand(CmdFinalityProvider())
	cmd.AddCommand(CmdFinalityProviders())

	return cmd
}

func CmdRegisteredConsumers() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registered-consumers",
		Short: "retrieve list of registered consumers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.ConsumerRegistryList(cmd.Context(), &types.QueryConsumerRegistryListRequest{
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "registered-consumers")

	return cmd
}

func CmdRegisteredConsumer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registered-consumer <consumer-id>",
		Short: "retrieve a given registered consumer's info",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ConsumersRegistry(
				cmd.Context(),
				&types.QueryConsumersRegistryRequest{
					ConsumerIds: []string{args[0]},
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

func CmdFinalityProviderConsumer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finality-provider-consumer <fp_btc_pk_hex>",
		Short: "retrieve a given CZ finality provider's registered consumer id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FinalityProviderConsumer(
				cmd.Context(),
				&types.QueryFinalityProviderConsumerRequest{
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
		Use:   "finality-provider <consumer-id> <fp_btc_pk_hex>",
		Short: "retrieve a given consumer's finality provider",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FinalityProvider(
				cmd.Context(),
				&types.QueryFinalityProviderRequest{
					FpBtcPkHex: args[1],
					ConsumerId: args[0],
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
		Use:   "finality-providers <consumer-id>",
		Short: "retrieve a given consumer's all finality providers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.FinalityProviders(cmd.Context(), &types.QueryFinalityProvidersRequest{
				ConsumerId: args[0],
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
