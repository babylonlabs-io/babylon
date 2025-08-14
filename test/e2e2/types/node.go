package types

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

const (
	BabylonHomePathInContainer = "/home/babylon/babylondata"
	FlagHome                   = "--home=" + BabylonHomePathInContainer
)

// Node represents a blockchain node enviroment in a docker container
type Node struct {
	Name  string
	Home  string
	Ports *NodePorts

	Container *Container
	Tm        *TestManager

	// Wallets all the wallets where the keyring files were created inside this node
	Wallets []*WalletSender
}

// ValidatorNode represents a validator node with additional capabilities
type ValidatorNode struct {
	*Node
	Wallet *ValidatorWallet
}

// NewValidatorNode creates a new validator node with simple ID generation
func NewValidatorNode(name string, cfg *ChainConfig) *ValidatorNode {
	n := NewNode(name, cfg)

	n.CreateKey(name)
	n.CreateConsensusKey

	return &ValidatorNode{
		Node:   n,
		Wallet: &ValidatorWallet{},
	}
}

// NewNode creates a new regular node with simple ID generation
func NewNode(name string, cfg *ChainConfig) *Node {
	// nodeID := fmt.Sprintf("%s-node-%s", cfg.ChainID, name)

	return &Node{
		Name: name,
		Home: filepath.Join(cfg.Home, name),
		// Container: ,
	}
}

// Node implementation
func (n *Node) Start() {

	// init node data first
	// func (cb *CurrentBranchConfigurer) ConfigureChain(chainConfig *chain.Config) error {
	// 	cb.t.Logf("starting e2e infrastructure from current branch for chain-id: %s", chainConfig.Id)
	// 	tmpDir, err := os.MkdirTemp("", "bbn-e2e-testnet-*")
	// 	if err != nil {
	// 		return err
	// 	}
	n.RunNodeResource()

}

// func newNode(chain *internalChain, nodeConfig *NodeConfig, gasLimit int64) (*internalNode, error) {
// 	node := &internalNode{
// 		chain:       chain,
// 		moniker:     fmt.Sprintf("%s-node-%s", chain.chainMeta.Id, nodeConfig.Name),
// 		isValidator: nodeConfig.IsValidator,
// 	}
// 	// creating keys comes before init
// 	if err := node.createKey(ValidatorWalletName); err != nil {
// 		return nil, err
// 	}
// 	if err := node.createConsensusKey(); err != nil {
// 		return nil, err
// 	}
// 	// generate genesis files
// 	if err := node.init(gasLimit); err != nil {
// 		return nil, err
// 	}
// 	if err := node.createNodeKey(); err != nil {
// 		return nil, err
// 	}
// 	node.createAppConfig(nodeConfig)
// 	return node, nil
// }

func (n *Node) T() *testing.T {
	return n.Tm.T
}

func (n *Node) CreateKey(keyName string) *WalletSender {
	nw := NewWalletSender(keyName, n)
	if n.IsChainRunning() {
		// set seq and acc number
	}
	n.Wallets = append(n.Wallets, nw)
	return nw
}

func (n *ValidatorNode) CreateConsensusKey() {
	consKey, err := CreateConsensusKey(n.Name, n.Wallet.Mnemonic, n.Home)
	require.NoError(n.T(), err)
	
}

func (n *Node) IsChainRunning() bool {
	return false
}

func (n *Node) RunNodeResource() {
	pwd, err := os.Getwd()
	require.NoError(n.T(), err)

	runOpts := &dockertest.RunOptions{
		Name:       n.Container.Name,
		Repository: n.Container.Repository,
		Tag:        n.Container.Tag,
		NetworkID:  n.Tm.NetworkID,
		User:       "root:root",
		Entrypoint: []string{
			"sh",
			"-c",
			// Use the following for debugging purposes:x
			// "export BABYLON_BLS_PASSWORD=password && babylond start " + FlagHome + " --log_level trace --trace",
			"export BABYLON_BLS_PASSWORD=password && babylond start " + FlagHome,
		},
		ExposedPorts: n.Ports.ContainerExposedPorts(),
		Mounts: []string{
			fmt.Sprintf("%s/:%s", n.Home, BabylonHomePathInContainer),
			fmt.Sprintf("%s/bytecode:/bytecode", pwd),
			fmt.Sprintf("%s/govProps:/govProps", pwd),
		},
	}

	resource, err := n.Tm.ContainerManager.Pool.RunWithOptions(runOpts, NoRestart)
	require.NoError(n.T(), err)

	n.Tm.ContainerManager.Resources[n.Container.Name] = resource
}

func (n *Node) Stop() error {

	return nil
}

func (n *Node) GetRPCAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("http://localhost:%d", n.Ports.RPC)
}

func (n *Node) GetP2PAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("tcp://localhost:%d", n.Ports.P2P)
}

func (n *Node) GetGRPCAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("localhost:%d", n.Ports.GRPC)
}

func (n *Node) GetRESTAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("http://localhost:%d", n.Ports.REST)
}

func (n *Node) IsHealthy() bool {
	// Implementation will be added later
	return true
}

func (n *Node) WaitForHeight(height int64) error {
	// Implementation will be added later
	return nil
}

func (n *Node) QueryHeight() (int64, error) {
	// Implementation will be added later
	return 0, nil
}

// generateTestID creates a unique test identifier
func GenerateTestID(testName string) string {
	sanitized := SanitizeTestName(testName)
	timestamp := time.Now().Unix()
	random := rand.Intn(10000)

	return fmt.Sprintf("%s-%d-%d", sanitized, timestamp, random)
}

func NoRestart(config *docker.HostConfig) {
	// in this case we don't want the nodes to restart on failure
	config.RestartPolicy = docker.RestartPolicy{
		Name: "no",
	}
}

func RunCommand(command string) error {
	fmt.Printf("Running command %s...\n", command)
	cmd := exec.Command(command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ImageExistsLocally(imageName string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName)
	return cmd.Run() == nil
}
