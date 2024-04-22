package chain

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"

	"github.com/babylonchain/babylon/test/e2e/util"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

func (n *NodeConfig) QueryChainRegistryList(pagination *query.PageRequest) *bsctypes.QueryChainRegistryListResponse {
	queryParams := url.Values{}
	if pagination != nil {
		queryParams.Set("pagination.key", base64.URLEncoding.EncodeToString(pagination.Key))
		queryParams.Set("pagination.limit", strconv.Itoa(int(pagination.Limit)))
	}

	bz, err := n.QueryGRPCGateway("/babylon/btcstkconsumer/v1/chain_registry_list", queryParams)
	require.NoError(n.t, err)

	var resp bsctypes.QueryChainRegistryListResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

func (n *NodeConfig) QueryChainRegistry(consumerChainId string) []*bsctypes.ChainRegister {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/chains_registry/%s", consumerChainId)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bsctypes.QueryChainsRegistryResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.ChainsRegister
}

func (n *NodeConfig) QueryConsumerFinalityProviders(consumerChainId string) []*bsctypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/finality_providers/%s", consumerChainId)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bsctypes.QueryFinalityProvidersResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProviders
}

func (n *NodeConfig) QueryConsumerFinalityProvider(consumerChainId, fpBtcPkHex string) *bsctypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstkconsumer/v1/finality_provider/%s/%s", consumerChainId, fpBtcPkHex)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bsctypes.QueryFinalityProviderResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProvider
}
