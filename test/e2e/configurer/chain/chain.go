package chain

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	govv1 "cosmossdk.io/api/cosmos/gov/v1"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer/config"
	"github.com/babylonlabs-io/babylon/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
)

type Config struct {
	initialization.ChainMeta

	ValidatorInitConfigs []*initialization.NodeConfig
	// voting period is number of blocks it takes to deposit, 1.2 seconds per validator to vote on the prop, and a buffer.
	VotingPeriod          float32
	ExpeditedVotingPeriod float32
	// upgrade proposal height for chain.
	UpgradePropHeight    int64
	LatestProposalNumber int
	LatestLockNumber     int
	NodeConfigs          []*NodeConfig
	BTCHeaders           []*btclighttypes.BTCHeaderInfo
	IBCConfig            *ibctesting.ChannelConfig

	LatestCodeId int

	t                *testing.T
	containerManager *containers.Manager
}

const (
	// defaultNodeIndex to use for querying and executing transactions.
	// It is used when we are indifferent about the node we are working with.
	defaultNodeIndex = 0
	// waitUntilRepeatPauseTime is the time to wait between each check of the node status.
	waitUntilRepeatPauseTime = 2 * time.Second
	// waitUntilrepeatMax is the maximum number of times to repeat the wait until condition.
	waitUntilrepeatMax = 60
)

func New(t *testing.T, containerManager *containers.Manager, id string, initValidatorConfigs []*initialization.NodeConfig, ibcConfig *ibctesting.ChannelConfig) *Config {
	numVal := float32(len(initValidatorConfigs))
	return &Config{
		ChainMeta: initialization.ChainMeta{
			Id: id,
		},
		ValidatorInitConfigs:  initValidatorConfigs,
		IBCConfig:             ibcConfig,
		VotingPeriod:          config.PropDepositBlocks + numVal*config.PropVoteBlocks + config.PropBufferBlocks,
		ExpeditedVotingPeriod: config.PropDepositBlocks + numVal*config.PropVoteBlocks + config.PropBufferBlocks - 2,
		t:                     t,
		containerManager:      containerManager,
		BTCHeaders:            []*btclighttypes.BTCHeaderInfo{},
	}
}

// CreateNode returns new initialized NodeConfig.
func (c *Config) CreateNode(initNode *initialization.Node) *NodeConfig {
	nodeConfig := &NodeConfig{
		Node:             *initNode,
		chainId:          c.ChainMeta.Id,
		containerManager: c.containerManager,
		t:                c.t,
	}
	c.NodeConfigs = append(c.NodeConfigs, nodeConfig)
	return nodeConfig
}

// RemoveNode removes node and stops it from running.
func (c *Config) RemoveNode(nodeName string) error {
	for i, node := range c.NodeConfigs {
		if node.Name == nodeName {
			c.NodeConfigs = append(c.NodeConfigs[:i], c.NodeConfigs[i+1:]...)
			return node.Stop()
		}
	}
	return fmt.Errorf("node %s not found", nodeName)
}

// WaitUntilHeight waits for all validators to reach the specified height at the minimum.
// returns error, if any.
func (c *Config) WaitUntilHeight(height int64) {
	// Ensure the nodes are making progress.
	doneCondition := func(syncInfo coretypes.SyncInfo) bool {
		curHeight := syncInfo.LatestBlockHeight

		if curHeight < height {
			c.t.Logf("current block height is %d, waiting to reach: %d", curHeight, height)
			return false
		}

		return !syncInfo.CatchingUp
	}

	for _, node := range c.NodeConfigs {
		c.t.Logf("node container: %s, waiting to reach height %d", node.Name, height)
		node.WaitUntil(doneCondition)
	}
}

// WaitForNumHeights waits for all nodes to go through a given number of heights.
func (c *Config) WaitForNumHeights(heightsToWait int64) {
	node, err := c.GetDefaultNode()
	require.NoError(c.t, err)
	currentHeight, err := node.QueryCurrentHeight()
	require.NoError(c.t, err)
	c.WaitUntilHeight(currentHeight + heightsToWait)
}

func (c *Config) SendIBC(dstChain *Config, recipient string, token sdk.Coin) {
	c.t.Logf("IBC sending %s from %s to %s (%s)", token, c.Id, dstChain.Id, recipient)

	dstNode, err := dstChain.GetDefaultNode()
	require.NoError(c.t, err)

	balancesDstPre, err := dstNode.QueryBalances(recipient)
	require.NoError(c.t, err)

	cmd := []string{"hermes", "tx", "raw", "ft-transfer", dstChain.Id, c.Id, "transfer", "channel-0", token.Amount.String(), fmt.Sprintf("--denom=%s", token.Denom), fmt.Sprintf("--receiver=%s", recipient), "--timeout-height-offset=1000"}
	_, _, err = c.containerManager.ExecHermesCmd(c.t, cmd, "Success")
	require.NoError(c.t, err)

	require.Eventually(
		c.t,
		func() bool {
			balancesDstPost, err := dstNode.QueryBalances(recipient)
			require.NoError(c.t, err)
			ibcCoin := balancesDstPost.Sub(balancesDstPre...)
			if ibcCoin.Len() == 1 {
				tokenPre := balancesDstPre.AmountOfNoDenomValidation(ibcCoin[0].Denom)
				tokenPost := balancesDstPost.AmountOfNoDenomValidation(ibcCoin[0].Denom)
				resPre := token.Amount
				resPost := tokenPost.Sub(tokenPre)
				return resPost.Uint64() == resPre.Uint64()
			} else {
				return false
			}
		},
		5*time.Minute,
		time.Second,
		"tx not received on destination chain",
	)

	c.t.Log("successfully sent IBC tokens")
}

// GetDefaultNode returns the default node of the chain.
// The default node is the first one created. Returns error if no
// nodes created.
func (c *Config) GetDefaultNode() (*NodeConfig, error) {
	return c.GetNodeAtIndex(defaultNodeIndex)
}

// GetPersistentPeers returns persistent peers from every node
// associated with a chain.
func (c *Config) GetPersistentPeers() []string {
	peers := make([]string, len(c.NodeConfigs))
	for i, node := range c.NodeConfigs {
		peers[i] = node.PeerId
	}
	return peers
}

func (c *Config) GetNodeAtIndex(nodeIndex int) (*NodeConfig, error) {
	if nodeIndex > len(c.NodeConfigs) {
		return nil, fmt.Errorf("node index (%d) is greter than the number of nodes available (%d)", nodeIndex, len(c.NodeConfigs))
	}
	return c.NodeConfigs[nodeIndex], nil
}

// TxGovVoteFromAllNodes votes in a gov prop from all nodes wallet.
func (c *Config) TxGovVoteFromAllNodes(propID int, option govv1.VoteOption, overallFlags ...string) {
	for _, n := range c.NodeConfigs {
		n.TxGovVote(n.WalletName, propID, option, overallFlags...)
	}
}

// BTCHeaderBytesHexJoined join all the btc headers as byte string hex
func (c *Config) BTCHeaderBytesHexJoined() string {
	if c.BTCHeaders == nil || len(c.BTCHeaders) == 0 {
		return ""
	}

	strBtcHeaders := make([]string, len(c.BTCHeaders))
	for i, btcHeader := range c.BTCHeaders {
		bz, err := btcHeader.Marshal()
		if err != nil {
			panic(err)
		}
		strBtcHeaders[i] = hex.EncodeToString(bz)
	}
	return strings.Join(strBtcHeaders, ",")
}
