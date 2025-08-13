package types

import (
	"fmt"
	"path/filepath"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	CHAIN_ID_BABYLON = "bbn"
	CHAIN_ID_BSN     = "consumer-bsn"
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

// InitGenesis holds genesis configuration
type InitGenesis struct {
	ChainConfig   *ChainConfig
	GenesisTime   time.Time
	VotingPeriod  time.Duration
	InitialTokens sdk.Coins
}

// Chain represents a blockchain with multiple nodes
type Chain struct {
	ID         string
	Nodes      []*Node
	Validators []*ValidatorNode
	Tm         *TestManager
	Genesis    *InitGenesis
	Config     *ChainConfig
}

// NewChainConfig creates a new chain configuration with default values
func NewChainConfig(tempDir, chainID string) *ChainConfig {
	return &ChainConfig{
		ChainID:               chainID,
		Home:                  filepath.Join(tempDir, chainID),
		ValidatorCount:        1,
		NodeCount:             0,
		BlockTime:             5 * time.Second,
		EpochLength:           10,
		VotingPeriod:          30 * time.Second,
		ExpeditedVotingPeriod: 15 * time.Second,
		BTCConfirmationDepth:  6,
		GasLimit:              300_000_000,
	}
}

// NewChain creates a new chain with the given configuration
func NewChain(tm *TestManager, cfg *ChainConfig) *Chain {
	nodes := make([]*Node, cfg.NodeCount)
	for i := 0; i < cfg.NodeCount; i++ {
		nodes[i] = NewNode(fmt.Sprintf("n-%d", i), cfg)
	}

	vals := make([]*ValidatorNode, cfg.ValidatorCount)
	for i := 0; i < cfg.ValidatorCount; i++ {
		vals[i] = NewValidatorNode(fmt.Sprintf("val-%d", i), cfg)
	}

	c := &Chain{
		Nodes:      nodes,
		Validators: vals,
		Tm:         tm,
		Config:     cfg,
		Genesis: &InitGenesis{
			ChainConfig:  cfg,
			GenesisTime:  time.Now(),
			VotingPeriod: cfg.VotingPeriod,
		},
	}

	return c
}

func (c *Chain) ChainsWaitUntilHeight(blkHeight uint32) {

}

// AllNodes returns an combined slice of validators and regular nodes
func (c *Chain) AllNodes() []*Node {
	ret := make([]*Node, c.Config.NodeCount+c.Config.ValidatorCount)
	for i, v := range c.Validators {
		ret[i] = v.Node
	}
	return append(ret, c.Nodes...)
}

// AddNode adds a regular node to the chain
func (c *Chain) AddNode(node *Node) {
	c.Nodes = append(c.Nodes, node)
}

// AddValidator adds a validator node to the chain
func (c *Chain) AddValidator(validator *ValidatorNode) {
	c.Validators = append(c.Validators, validator)
}

// Start starts all nodes in the chain
func (c *Chain) Start() {
	for _, n := range c.AllNodes() {
		n.Start()
	}
}

// GetValidators returns all validator nodes
func (c *Chain) GetValidators() []*ValidatorNode {
	return c.Validators
}

// GetChainID returns the chain ID
func (c *Chain) GetChainID() string {
	return c.Config.ChainID
}
