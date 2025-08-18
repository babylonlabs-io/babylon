package types

import (
	"context"
	"net/url"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
)

func (n *Node) LatestBlockNumber() (uint64, error) {
	status, err := n.RpcClient.Status(context.Background())
	if err != nil {
		return 0, err
	}
	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

// QueryIBCChannels queries all IBC channels on this node
func (n *Node) QueryIBCChannels() *channeltypes.QueryChannelsResponse {
	path := "/ibc/core/channel/v1/channels"
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.T(), err)

	var resp channeltypes.QueryChannelsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.T(), err)

	return &resp
}
