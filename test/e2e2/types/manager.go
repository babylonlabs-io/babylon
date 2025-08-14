package types

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
)

// TestManager manages isolated Docker networks for tests
type TestManager struct {
	T *testing.T

	NetworkID        string
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

	networkID := GenerateNetworkID(t)
	network, err := pool.CreateNetwork(fmt.Sprintf("bbn-e2e-%s", networkID))
	if err != nil {
		t.Fatalf("failed to create Docker network: %v", err)
	}

	containerManager := &ContainerManager{
		Pool:      pool,
		Network:   network,
		Resources: make(map[string]*dockertest.Resource),
	}

	nm := &TestManager{
		NetworkID:        networkID,
		TempDir:          t.TempDir(),
		PortMgr:          NewPortManager(),
		Pool:             pool,
		Network:          network,
		ContainerManager: containerManager,
		T:                t,
	}

	// Add network cleanup - this will be called last
	t.Cleanup(func() {
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

// Start runs all the nodes
func (tm *TestManager) Start() {
	for _, chain := range tm.Chains {
		chain.Start()
	}
}

// Start runs all the nodes and the hermes relayer with an ics20 transfer channel
func (tm *TestManagerIbc) Start() {
	tm.TestManager.Start()

	tm.ChainsWaitUntilHeight(1)

	tm.Hermes.Start()
}

func (tm *TestManager) ChainsWaitUntilHeight(blkHeight uint32) {
	for _, chain := range tm.Chains {
		chain.WaitUntilBlkHeight(blkHeight)
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
