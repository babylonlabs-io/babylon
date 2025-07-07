package query

import (
	"context"

	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
)

// QueryBTCStaking queries the BTC staking consumer module of the Babylon node according to the given function
func (c *QueryClient) QueryBTCStkConsumer(f func(ctx context.Context, queryClient bsctypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := bsctypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

// QueryConsumerRegistryList returns a list of the consumer systems
func (c *QueryClient) QueryConsumerRegistryList(pagination *sdkquerytypes.PageRequest) (*bsctypes.QueryConsumerRegistryListResponse, error) {
	var resp *bsctypes.QueryConsumerRegistryListResponse
	err := c.QueryBTCStkConsumer(func(ctx context.Context, queryClient bsctypes.QueryClient) error {
		var err error
		req := &bsctypes.QueryConsumerRegistryListRequest{
			Pagination: pagination,
		}
		resp, err = queryClient.ConsumerRegistryList(ctx, req)
		return err
	})

	return resp, err
}

// QueryConsumersRegistry returns the consumer systems with the given consumer IDs
func (c *QueryClient) QueryConsumersRegistry(consumerIDs []string) (*bsctypes.QueryConsumersRegistryResponse, error) {
	var resp *bsctypes.QueryConsumersRegistryResponse
	err := c.QueryBTCStkConsumer(func(ctx context.Context, queryClient bsctypes.QueryClient) error {
		var err error
		req := &bsctypes.QueryConsumersRegistryRequest{
			ConsumerIds: consumerIDs,
		}
		resp, err = queryClient.ConsumersRegistry(ctx, req)
		return err
	})

	return resp, err
}
