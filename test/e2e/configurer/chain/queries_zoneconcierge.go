package chain

import (
	"net/url"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	zoneconciergetype "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

func (n *NodeConfig) QueryZoneConciergeParams() *zoneconciergetype.Params {
	bz, err := n.QueryGRPCGateway("/babylon/zoneconcierge/v1/params", url.Values{})
	require.NoError(n.t, err)

	var resp zoneconciergetype.QueryParamsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp.Params
}
