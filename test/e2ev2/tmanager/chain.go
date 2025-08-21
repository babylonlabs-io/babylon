package tmanager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	appkeepers "github.com/babylonlabs-io/babylon/v3/app/keepers"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
)

const (
	CHAIN_ID_BABYLON = "bbn"
	CHAIN_ID_BSN     = "consumer-bsn"
)

var (
	BabyInitialBalance = sdkmath.NewInt(1_000_000_000000) // 1kk ubbn
)

// ChainConfig defines configuration for a blockchain
type ChainConfig struct {
	ChainID               string
	Home                  string
	ValidatorCount        int
	NodeCount             int
	BlockTime             time.Duration
	EpochLength           int64
	VotingPeriod          time.Duration
	ExpeditedVotingPeriod time.Duration
	BTCConfirmationDepth  int
	GasLimit              int64
}

// Chain represents a blockchain with multiple nodes
type Chain struct {
	Nodes          []*Node
	Validators     []*ValidatorNode
	Tm             *TestManager
	InitialGenesis *InitGenesis
	Config         *ChainConfig
}

// NewChainConfig creates a new chain configuration with default values
func NewChainConfig(tempDir, chainID string) *ChainConfig {
	return &ChainConfig{
		ChainID: chainID,
		Home:    filepath.Join(tempDir, chainID),
		// starts with 2 nodes (1 val, one non-validator)
		ValidatorCount:        1,
		NodeCount:             1,
		BlockTime:             2 * time.Second,
		EpochLength:           10,
		VotingPeriod:          12 * time.Second,
		ExpeditedVotingPeriod: 6 * time.Second,
		BTCConfirmationDepth:  6,
		GasLimit:              300_000_000,
	}
}

// NewChain creates a new chain with the given configuration
func NewChain(tm *TestManager, cfg *ChainConfig) *Chain {
	require.GreaterOrEqual(tm.T, cfg.NodeCount+cfg.ValidatorCount, 1)

	nodes := make([]*Node, cfg.NodeCount)
	for i := 0; i < cfg.NodeCount; i++ {
		nodes[i] = NewNode(tm, fmt.Sprintf("n-%d", i), cfg)
	}

	vals := make([]*ValidatorNode, cfg.ValidatorCount)
	for i := 0; i < cfg.ValidatorCount; i++ {
		vals[i] = NewValidatorNode(tm, fmt.Sprintf("val-%d", i), cfg)
	}

	initialTokens := datagen.GenRandomCoins(tm.R).MulInt(sdkmath.NewInt(10))
	initialTokens = initialTokens.Add(sdk.NewCoin(appparams.DefaultBondDenom, BabyInitialBalance))

	c := &Chain{
		Nodes:      nodes,
		Validators: vals,
		Tm:         tm,
		Config:     cfg,
		InitialGenesis: &InitGenesis{
			ChainConfig:           cfg,
			GenesisTime:           time.Now(),
			VotingPeriod:          cfg.VotingPeriod,
			ExpeditedVotingPeriod: cfg.ExpeditedVotingPeriod,
			InitialTokens:         initialTokens,
		},
	}

	c.InitGenesis()
	c.WritePeers()

	return c
}

func (c *Chain) ChainID() string {
	return c.Config.ChainID
}

func (c *Chain) T() *testing.T {
	return c.Tm.T
}

func (c *Chain) InitGenesis() {
	nodes := c.AllNodes()

	// gets the first node genesis, later we should write the same genesis in all nodes
	firstNode := nodes[0]

	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(firstNode.Home)

	genFilePath := config.GenesisFile()
	appGenState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFilePath)
	require.NoError(c.T(), err)

	balancesToAdd := make([]banktypes.Balance, 0)
	accsToAdd := make([]*authtypes.BaseAccount, 0)
	for _, n := range nodes {
		for _, nw := range n.Wallets {
			wBalance := banktypes.Balance{Address: nw.Addr(), Coins: c.InitialGenesis.InitialTokens}
			balancesToAdd = append(balancesToAdd, wBalance)

			genAccount := authtypes.NewBaseAccount(nw.Address, nw.PrivKey.PubKey(), 0, 0)
			accsToAdd = append(accsToAdd, genAccount)
		}
	}

	sanitizedAccs, err := UpdateGenAccounts(appGenState, accsToAdd)
	require.NoError(c.T(), err, "failed to set gen accs")
	// Update sequence and account numbers for wallets based on sanitized accounts
	c.UpdateWalletSequenceAndAccountNumbers(sanitizedAccs)

	// update all other modules
	err = UpdateGenModulesState(appGenState, *c.InitialGenesis, c.Validators, nil, nil, balancesToAdd)
	require.NoError(c.T(), err, "failed to update gen state for all other modules")

	appStateJSON, err := json.Marshal(appGenState)
	require.NoError(c.T(), err, "failed to marshal application genesis state")
	genDoc.AppState = appStateJSON

	// write the same genesis to all nodes
	c.WriteGenesis(genDoc)
}

// UpdateWalletSequenceAndAccountNumbers updates the sequence and account numbers for all wallets
// based on the sanitized accounts from genesis
func (c *Chain) UpdateWalletSequenceAndAccountNumbers(sanitizedAccs authtypes.GenesisAccounts) {
	// Create a map of address to account for quick lookup
	accMap := make(map[string]sdk.AccountI)
	for _, acc := range sanitizedAccs {
		accMap[acc.GetAddress().String()] = acc
	}

	// Update wallet properties based on the sanitized accounts
	for _, node := range c.AllNodes() {
		for _, wallet := range node.Wallets {
			acc, exists := accMap[wallet.Addr()]
			if !exists {
				continue
			}
			wallet.AccountNumber = acc.GetAccountNumber()
			wallet.SequenceNumber = acc.GetSequence()
		}
	}
}

func (c *Chain) WritePeers() {
	var peers []string
	allNodes := c.AllNodes()
	for _, n := range allNodes {
		peers = append(peers, n.PeerID)
	}

	for _, n := range allNodes {
		n.InitConfigWithPeers(peers)
	}

	for _, n := range allNodes {
		_, err := appkeepers.CreateClientConfig(c.Config.ChainID, keyring.BackendTest, n.Home)
		require.NoError(c.T(), err)
	}
}

func (c *Chain) WaitUntilBlkHeight(blkHeight uint32) {
	for _, n := range c.AllNodes() {
		n.WaitUntilBlkHeight(blkHeight)
	}
}

// AllNodes returns an combined slice of validators and regular nodes
func (c *Chain) AllNodes() []*Node {
	ret := make([]*Node, c.Config.NodeCount+c.Config.ValidatorCount)
	iter := 0
	for _, v := range c.Validators {
		ret[iter] = v.Node
		iter++
	}
	for _, n := range c.Nodes {
		ret[iter] = n
		iter++
	}
	return ret
}

// WriteGenesis writes the genesis file in all the nodes available
func (c *Chain) WriteGenesis(genDoc *genutiltypes.AppGenesis) {
	for _, n := range c.AllNodes() {
		n.WriteGenesis(genDoc)
	}
}

// Start starts all nodes in the chain
func (c *Chain) Start() {
	for _, n := range c.AllNodes() {
		n.Start()
	}
}
