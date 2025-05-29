package chain

import (
	"fmt"
	"net/url"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	bstkconsumertypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

func (n *NodeConfig) QueryBTCStkConsumerParams() *bstkconsumertypes.Params {
	bz, err := n.QueryGRPCGateway("/babylon/btcstkconsumer/v1/params", url.Values{})
	require.NoError(n.t, err)

	var resp bstkconsumertypes.QueryParamsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp.Params
}

func (n *NodeConfig) QueryBTCStkConsumerConsumers() *[]*bstkconsumertypes.ConsumerRegisterResponse {
	bz, err := n.QueryGRPCGateway("/babylon/btcstkconsumer/v1/consumer_registry_list", url.Values{})
	require.NoError(n.t, err)

	var resp bstkconsumertypes.QueryConsumerRegistryListResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp.ConsumerRegisters
}

func (n *NodeConfig) QueryBTCStkConsumerConsumer(consumerID string) *bstkconsumertypes.QueryConsumersRegistryResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/consumers_registry/%s", consumerID)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstkconsumertypes.QueryConsumersRegistryResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

func (n *NodeConfig) QueryConsumerFinalityProviders(consumerId string) []*bstkconsumertypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/finality_providers/%s", consumerId)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstkconsumertypes.QueryFinalityProvidersResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProviders
}

func (n *NodeConfig) QueryConsumerFinalityProvider(consumerID, btcPkHex string) *bstkconsumertypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/finality_provider/%s/%s", consumerID, btcPkHex)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstkconsumertypes.QueryFinalityProviderResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProvider
}

func (n *NodeConfig) QueryConsumerFinalityProviderConsumer(btcPkHex string) *bstkconsumertypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/finality_provider_consumer/%s", btcPkHex)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstkconsumertypes.QueryFinalityProviderResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProvider
}
