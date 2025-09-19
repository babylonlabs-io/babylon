package chain

import (
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/types/query"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
)

func (n *NodeConfig) QueryBtcLightClientMainchain(pagination *query.PageRequest) (*btclighttypes.QueryMainChainResponse, error) {
	queryParams := url.Values{}
	if pagination != nil {
		queryParams.Set("pagination.key", base64.URLEncoding.EncodeToString(pagination.Key))
		queryParams.Set("pagination.limit", strconv.Itoa(int(pagination.Limit)))
	}

	bz, err := n.QueryGRPCGateway("/babylon/btclightclient/v1/mainchain", queryParams)
	if err != nil {
		return nil, err
	}

	var resp btclighttypes.QueryMainChainResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (n *NodeConfig) QueryBtcLightClientMainchainAll() []*btclighttypes.BTCHeaderInfoResponse {
	headers := make([]*btclighttypes.BTCHeaderInfoResponse, 0)

	limit := uint64(1000)
	pagination := &sdkquerytypes.PageRequest{
		Limit: limit,
	}
	for {
		resp, err := n.QueryBtcLightClientMainchain(pagination)
		if err != nil {
			if strings.Contains(err.Error(), "header specified by key does not exist") {
				// err could come as {"code":3,"message":"header specified by key does not exist","details":[]}
				return headers
			}
			require.NoError(n.t, err)
		}

		headers = append(headers, resp.Headers...)
		if len(resp.Headers) != int(limit) {
			return headers
		}
		pagination.Key = resp.Pagination.NextKey
	}
}
