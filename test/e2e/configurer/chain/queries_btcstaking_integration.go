package chain

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"

	"github.com/babylonlabs-io/babylon/test/e2e/util"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

func (n *NodeConfig) QueryConsumerRegistryList(pagination *query.PageRequest) *bsctypes.QueryConsumerRegistryListResponse {
	queryParams := url.Values{}
	if pagination != nil {
		queryParams.Set("pagination.key", base64.URLEncoding.EncodeToString(pagination.Key))
		queryParams.Set("pagination.limit", strconv.Itoa(int(pagination.Limit)))
	}

	bz, err := n.QueryGRPCGateway("/babylon/btcstkconsumer/v1/consumer_registry_list", queryParams)
	require.NoError(n.t, err)

	var resp bsctypes.QueryConsumerRegistryListResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

func (n *NodeConfig) QueryConsumerRegistry(consumerId string) []*bsctypes.ConsumerRegister {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/consumers_registry/%s", consumerId)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bsctypes.QueryConsumersRegistryResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.ConsumersRegister
}

func (n *NodeConfig) QueryConsumerFinalityProviders(consumerId string) []*bsctypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/finality_providers/%s", consumerId)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bsctypes.QueryFinalityProvidersResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProviders
}

func (n *NodeConfig) QueryConsumerFinalityProvider(consumerId, fpBtcPkHex string) *bsctypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/finality_provider/%s/%s", consumerId, fpBtcPkHex)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bsctypes.QueryFinalityProviderResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProvider
}
