package chain

import (
	"net/url"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/test/e2e/util"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
)

func (n *NodeConfig) QueryBtcLightClientMainchain() []*btclighttypes.BTCHeaderInfoResponse {
	bz, err := n.QueryGRPCGateway("/babylon/btclightclient/v1/mainchain", url.Values{})
	require.NoError(n.t, err)

	var resp btclighttypes.QueryMainChainResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.Headers
}
