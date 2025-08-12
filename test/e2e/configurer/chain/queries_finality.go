package chain

import (
	"fmt"
	"net/url"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/util"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
	"github.com/stretchr/testify/require"
)

func (n *NodeConfig) QueryFinalityProviderPowerAtHeight(fpBTCPK *bbn.BIP340PubKey, blkHeight uint64) uint64 {
	path := fmt.Sprintf("/babylon/finality/v1/finality_providers/%s/power/%d", fpBTCPK.MarshalHex(), blkHeight)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryFinalityProviderPowerAtHeightResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.VotingPower
}

func (n *NodeConfig) QueryVotesAtHeight(height uint64) []bbn.BIP340PubKey {
	path := fmt.Sprintf("/babylon/finality/v1/votes/%d", height)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryVotesAtHeightResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.BtcPks
}
