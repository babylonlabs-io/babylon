package query

import (
	"context"

	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
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

// QueryConsumerFinalityProviders returns a list of finality providers under the given consumer system
func (c *QueryClient) QueryConsumerFinalityProviders(consumerID string, pagination *sdkquerytypes.PageRequest) (*bsctypes.QueryFinalityProvidersResponse, error) {
	var resp *bsctypes.QueryFinalityProvidersResponse
	err := c.QueryBTCStkConsumer(func(ctx context.Context, queryClient bsctypes.QueryClient) error {
		var err error
		req := &bsctypes.QueryFinalityProvidersRequest{
			ConsumerId: consumerID,
			Pagination: pagination,
		}
		resp, err = queryClient.FinalityProviders(ctx, req)
		return err
	})

	return resp, err
}

// QueryConsumerFinalityProvider returns the finality provider with the given BTC PK under
// the given consumer ID
func (c *QueryClient) QueryConsumerFinalityProvider(consumerID string, fpBTCPkHex string) (*bsctypes.FinalityProviderResponse, error) {
	var resp *bsctypes.QueryFinalityProviderResponse
	err := c.QueryBTCStkConsumer(func(ctx context.Context, queryClient bsctypes.QueryClient) error {
		var err error
		req := &bsctypes.QueryFinalityProviderRequest{
			ConsumerId: consumerID,
			FpBtcPkHex: fpBTCPkHex,
		}
		resp, err = queryClient.FinalityProvider(ctx, req)
		return err
	})

	return resp.FinalityProvider, err
}

// QueryFinalityProviderConsumer returns the consumer that a given finality provider
// belongs to
func (c *QueryClient) QueryFinalityProviderConsumer(fpBTCPkHex string) (string, error) {
	var resp *bsctypes.QueryFinalityProviderConsumerResponse
	err := c.QueryBTCStkConsumer(func(ctx context.Context, queryClient bsctypes.QueryClient) error {
		var err error
		req := &bsctypes.QueryFinalityProviderConsumerRequest{
			FpBtcPkHex: fpBTCPkHex,
		}
		resp, err = queryClient.FinalityProviderConsumer(ctx, req)
		return err
	})

	return resp.ConsumerId, err
}
