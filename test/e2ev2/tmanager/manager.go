package tmanager

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/test-go/testify/require"
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
	// LastTxsIDs       []string
}

// TestManagerIbc manages two chains with ibc connection
type TestManagerIbc struct {
	*TestManager
	Hermes *HermesRelayer
}

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
