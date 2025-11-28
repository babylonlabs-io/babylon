package tmanager

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	v5 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v5"
)

// TestManager manages isolated Docker networks for tests
type TestManager struct {
	T *testing.T
	R *rand.Rand

	TempDir          string
	PortMgr          *PortManager
	Pool             *dockertest.Pool
	Network          *dockertest.Network
	ContainerManager *ContainerManager
	Chains           map[string]*Chain
}

// TestManagerIbc manages two chains with ibc connection
type TestManagerIbc struct {
	*TestManager
	Hermes *HermesRelayer
}

// TestManagerUpgrade manages software upgrade, which includes proposal upgrade and fork upgrade
type TestManagerUpgrade struct {
	*TestManager
	ForkHeight int64 // ForkHeight > 0 implies that this is a fork upgrade, otherwise, proposal upgrade
}

type PreUpgradeFunc func([]*Node)

// NewTestManager creates a new network manager with isolated Docker network
func NewTestManager(t *testing.T) *TestManager {
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("failed to create Docker pool: %v", err)
	}

	network, err := pool.CreateNetwork(fmt.Sprintf("bbn-e2e-%s", GenerateNetworkID(t)))
	if err != nil {
		t.Fatalf("failed to create Docker network: %v", err)
	}

	containerManager := &ContainerManager{
		Pool:      pool,
		Network:   network,
		Resources: make(map[string]*dockertest.Resource),
	}

	nm := &TestManager{
		T:                t,
		R:                rand.New(rand.NewSource(time.Now().Unix())),
		TempDir:          t.TempDir(),
		PortMgr:          NewPortManager(),
		Pool:             pool,
		Network:          network,
		ContainerManager: containerManager,
		Chains:           make(map[string]*Chain),
	}

	// Add network cleanup - this will be called last
	t.Cleanup(func() {
		for name, r := range nm.ContainerManager.Resources {
			err = r.Close()
			if err != nil {
				t.Logf("error removing resource %s %+v", name, err)
			}
		}
		err = pool.RemoveNetwork(network)
		if err != nil {
			t.Logf("error removing network %+v", err)
		}
	})

	return nm
}

func NewTmWithIbc(t *testing.T) *TestManagerIbc {
	tm := NewTestManager(t)

	bbnCfg := NewChainConfig(tm.TempDir, CHAIN_ID_BABYLON)
	bsnCfg := NewChainConfig(tm.TempDir, CHAIN_ID_BSN)

	tm.Chains[CHAIN_ID_BABYLON] = NewChain(tm, bbnCfg)
	tm.Chains[CHAIN_ID_BSN] = NewChain(tm, bsnCfg)

	return &TestManagerIbc{
		TestManager: tm,
		Hermes:      NewHermesRelayer(tm),
	}
}

func NewTmWithUpgrade(
	t *testing.T,
	forkHeight int64,
	tag string,
) *TestManagerUpgrade {
	tm := NewTestManager(t)
	bbnCfg := NewChainConfig(tm.TempDir, CHAIN_ID_BABYLON)
	bbnCfg.IsUpgrade = true
	// if tag is empty string, use default tag v4.0.0-rc.1
	if tag == "" {
		tag = BabylonContainerTagBeforeUpgrade
	}
	bbnCfg.Tag = tag
	bbnCfg.BootstrapRepository = BabylonContainerNameBeforeUpgrade
	tm.Chains[CHAIN_ID_BABYLON] = NewChain(tm, bbnCfg)

	return &TestManagerUpgrade{
		TestManager: tm,
		ForkHeight:  forkHeight,
	}
}

func (tm *TestManager) NetworkID() string {
	return tm.Network.Network.ID
}

// Start runs all the nodes
func (tm *TestManager) Start() {
	for _, chain := range tm.Chains {
		chain.Start()
	}
}

// Start runs all the nodes and the hermes relayer with an ics20 transfer channel
func (tm *TestManagerIbc) Start() {
	tm.TestManager.Start()

	// Wait for chains to produce at least one block
	tm.ChainsWaitUntilHeight(1)

	cA, cB := tm.ChainBBN(), tm.ChainBSN()
	tm.Hermes.Start(cA, cB)

	tm.Hermes.CreateIBCTransferChannel(cA, cB)
	tm.ChainsWaitUntilNextBlock()
	tm.RequireChannelsCreated()

	// creating channels by hermes modifies the acc sequence
	tm.UpdateWalletsAccSeqNumber()
}

// Start runs all the nodes and wait for block 1
func (tm *TestManagerUpgrade) Start() {
	tm.TestManager.Start()

	// wait for chains to produce at least one block
	tm.ChainsWaitUntilHeight(1)
}

// Upgrade executes preUpgradeFunc and processes upgrade
// NOTE: this function must be invoked after Start()
func (tm *TestManagerUpgrade) Upgrade(govMsg *govtypes.MsgSubmitProposal, preUpgradeFunc PreUpgradeFunc) {
	var nodes []*Node
	for _, chain := range tm.Chains {
		nodes = append(nodes, chain.AllNodes()...)
	}
	preUpgradeFunc(nodes)

	// run upgrade either fork or proposal upgrade
	if tm.ForkHeight > 0 {
		tm.runForkUpgrade()
	} else {
		if err := tm.runProposalUpgrade(govMsg); err != nil {
			tm.T.Fatalf("failed to run proposal upgrade: %v", err)
		}
	}

	// check if the upgrade was applied
	for _, chain := range tm.Chains {
		for _, node := range chain.AllNodes() {
			height, err := node.LatestBlockNumber()
			if err != nil {
				tm.T.Fatalf("failed to get latest block height: %v", err)
			}
			tm.T.Logf("node %s: latest block height on chain %s: %d", node.Name, chain.ChainID(), height)
			appliedHeight := node.QueryAppliedPlan(v5.UpgradeName)
			tm.T.Logf("node %s: %s plan applied at height: %d", node.Name, v5.UpgradeName, appliedHeight)
		}
	}
}

// UpdateWalletsAccSeqNumber iterates over all chains, nodes and wallets
// to update the acc sequence and number
func (tm *TestManagerIbc) UpdateWalletsAccSeqNumber() {
	// Query and update account sequence and numbers for all wallets
	for _, chain := range tm.Chains {
		for _, node := range chain.AllNodes() {
			node.UpdateWalletsAccSeqNumber()
		}
	}
}

func (tm *TestManager) ChainsWaitUntilHeight(blkHeight uint32) {
	for _, chain := range tm.Chains {
		chain.WaitUntilBlkHeight(blkHeight)
	}
}

func (tm *TestManager) ChainsWaitUntilNextBlock() {
	for _, chain := range tm.Chains {
		chain.Nodes[0].WaitForNextBlock()
	}
}

func (tm *TestManager) ChainNodes() []*Node {
	var nodes []*Node
	for _, chain := range tm.Chains {
		nodes = append(nodes, chain.Nodes...)
	}
	return nodes
}

func (tm *TestManager) ChainValidator() *ValidatorNode {
	return tm.Chains[CHAIN_ID_BABYLON].Validators[0]
}

// GenerateNetworkID creates a unique network identifier for the test
func GenerateNetworkID(t *testing.T) string {
	// Use test name + timestamp + random to ensure uniqueness
	testName := SanitizeTestName(t.Name())
	timestamp := time.Now().Unix()
	random := rand.Intn(10000)

	return fmt.Sprintf("%s-%d-%d", testName, timestamp, random)
}

func (tm *TestManagerIbc) ChainBBN() *Chain {
	return tm.Chains[CHAIN_ID_BABYLON]
}

func (tm *TestManagerIbc) ChainBSN() *Chain {
	return tm.Chains[CHAIN_ID_BSN]
}

func (tm *TestManagerIbc) ChainNodes() (bbn, bsn *Node) {
	return tm.ChainBBN().Nodes[0], tm.ChainBSN().Nodes[0]
}

func (tm *TestManagerIbc) RequireChannelsCreated() {
	bbn, bsn := tm.ChainNodes()

	tm.T.Log("Verifying IBC channels were created...")
	bbnChannels := bbn.QueryIBCChannels()
	require.Len(tm.T, bbnChannels.Channels, 1, "No IBC channels found on Babylon chain")
	bsnChannels := bsn.QueryIBCChannels()
	require.Len(tm.T, bsnChannels.Channels, 1, "No IBC channels found on BSN Consumer chain")
}
