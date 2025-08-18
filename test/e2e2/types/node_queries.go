package types

import (
	"context"
)

func (n *Node) LatestBlockNumber() (uint64, error) {
	status, err := n.RpcClient.Status(context.Background())
	if err != nil {
		return 0, err
	}
	return uint64(status.SyncInfo.LatestBlockHeight), nil
}
