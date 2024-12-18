package initialization

import (
	"fmt"
	"time"

	appkeepers "github.com/babylonlabs-io/babylon/app/keepers"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
)

func InitChain(
	id, dataDir string,
	nodeConfigs []*NodeConfig,
	votingPeriod, expeditedVotingPeriod time.Duration,
	forkHeight int,
	btcHeaders []*btclighttypes.BTCHeaderInfo,
) (*Chain, error) {
	chain := new(id, dataDir)

	for _, nodeConfig := range nodeConfigs {
		newNode, err := newNode(chain, nodeConfig)
		if err != nil {
			return nil, err
		}
		chain.nodes = append(chain.nodes, newNode)
	}

	if err := initGenesis(chain, votingPeriod, expeditedVotingPeriod, forkHeight, btcHeaders); err != nil {
		return nil, err
	}

	var peers []string
	for _, peer := range chain.nodes {
		peerID := fmt.Sprintf("%s@%s:26656", peer.getNodeKey().ID(), peer.moniker)
		peer.peerId = peerID
		peers = append(peers, peerID)
	}

	for _, node := range chain.nodes {
		if err := node.initNodeConfigs(peers); err != nil {
			return nil, err
		}
	}

	for _, node := range chain.nodes {
		_, _ = appkeepers.CreateClientConfig(node.chain.chainMeta.Id, "test", node.configDir())
	}

	return chain.export(), nil
}
