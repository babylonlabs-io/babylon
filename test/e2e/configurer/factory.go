package configurer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/initialization"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclighttypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
)

type Configurer interface {
	ConfigureChains() error

	ClearResources() error

	GetChainConfig(chainIndex int) *chain.Config

	RunSetup() error

	RunValidators() error

	RunHermesRelayerIBC() error

	// RunCosmosRelayerIBC configures IBC with Go relayer
	RunCosmosRelayerIBC() error

	RunIBCTransferChannel() error
	// CompleteIBCChannelHandshake completes the channel handshake in cases when ChanOpenInit was initiated
	// by some transaction that was previously executed on the chain. For example,
	// ICA MsgRegisterInterchainAccount will perform ChanOpenInit during its execution.
	CompleteIBCChannelHandshake(srcChain, dstChain, srcConnection, dstConnection, srcPort, dstPort, srcChannel, dstChannel string) error
}

const (
	btcNetworkStr = string(bbn.BtcSimnet)
)

var (
	// Last nodes are non validator nodes to serve as the ones using relayer. Out
	// validators are constantly sending bls transactions which make relayer operations
	// fail constantly

	// each started validator container corresponds to one of
	// the configurations below.
	validatorConfigsChainA = []*initialization.NodeConfig{
		{
			// this is a node that is used to state-sync from so its snapshot-interval
			// is frequent.
			Name:               "babylon-default-a-1",
			Pruning:            "default",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   25,
			SnapshotKeepRecent: 10,
			IsValidator:        true,
			BtcNetwork:         btcNetworkStr,
		},
		{
			Name:               "babylon-default-a-2",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        true,
			BtcNetwork:         btcNetworkStr,
		},
		{
			Name:               "babylon-default-a-3",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        false,
			BtcNetwork:         btcNetworkStr,
		},
	}
	validatorConfigsChainB = []*initialization.NodeConfig{
		{
			Name:               "babylon-default-b-1",
			Pruning:            "default",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        true,
			BtcNetwork:         btcNetworkStr,
		},
		{
			Name:               "babylon-default-b-2",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        true,
			BtcNetwork:         btcNetworkStr,
		},
		{
			Name:               "babylon-default-b-3",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        false,
			BtcNetwork:         btcNetworkStr,
		},
	}
)

const MaxIndetifierSize = 10

// NewBTCTimestampingConfigurer returns a new Configurer for BTC timestamping service.
// TODO currently only one configuration is available. Consider testing upgrades
// when necessary
func NewBTCTimestampingConfigurer(t *testing.T, isDebugLogEnabled bool) (Configurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, false, false)
	if err != nil {
		return nil, err
	}

	return NewCurrentBranchConfigurer(t,
		[]*chain.Config{
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), nil),
			chain.New(t, containerManager, initialization.ChainBID, updateNodeConfigNameWithIdentifier(validatorConfigsChainB, identifier), nil),
		},
		withIBC(baseSetup), // base set up with IBC
		containerManager,
	), nil
}

func NewIBCTransferConfigurer(t *testing.T, isDebugLogEnabled bool) (Configurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, false, false)
	if err != nil {
		return nil, err
	}

	return NewCurrentBranchConfigurer(t,
		[]*chain.Config{
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), nil),
			chain.New(t, containerManager, initialization.ChainBID, updateNodeConfigNameWithIdentifier(validatorConfigsChainB, identifier), nil),
		},
		withIBCTransferChannel(baseSetup), // base set up with IBC
		containerManager,
	), nil
}

// NewBabylonConfigurer returns a new Configurer for BTC staking service
func NewBabylonConfigurer(t *testing.T, isDebugLogEnabled bool) (Configurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, false, false)
	if err != nil {
		return nil, err
	}

	return NewCurrentBranchConfigurer(t,
		[]*chain.Config{
			// we only need 1 chain for testing BTC staking
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), nil),
		},
		baseSetup, // base set up
		containerManager,
	), nil
}

// NewSoftwareUpgradeConfigurer returns a new Configurer for Software Upgrade testing
func NewSoftwareUpgradeConfigurer(t *testing.T, isDebugLogEnabled bool, upgradePath string, btcHeaders []*btclighttypes.BTCHeaderInfo, preUpgradeFunc PreUpgradeFunc) (*UpgradeConfigurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, false, true)
	if err != nil {
		return nil, err
	}

	return NewUpgradeConfigurer(t,
		[]*chain.Config{
			// we only need 1 chain for testing BTC staking
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), nil),
		},
		withUpgrade(baseSetup), // base set up with upgrade
		containerManager,
		upgradePath,
		0,
		preUpgradeFunc,
	), nil
}

// NewFinalityContractConfigurer returns a new Configurer for finality contract tests.
func NewFinalityContractConfigurer(t *testing.T, isDebugLogEnabled bool) (Configurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, false, false)
	if err != nil {
		return nil, err
	}

	return NewCurrentBranchConfigurer(t,
		[]*chain.Config{
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), nil),
		},
		baseSetup, // base setup
		containerManager,
	), nil
}

func identifierName(t *testing.T) string {
	str := strings.ToLower(t.Name())
	str = strings.ReplaceAll(str, "/", "-")
	h := sha256.New()
	hex := hex.EncodeToString(h.Sum([]byte(str)))
	if len(hex) > MaxIndetifierSize { // cap size to first MaxIndetifierSize
		return hex[:MaxIndetifierSize-1]
	}
	return hex
}

func updateNodeConfigNameWithIdentifier(cfgs []*initialization.NodeConfig, identifier string) []*initialization.NodeConfig {
	return updateNodeConfigs(cfgs, func(cfg *initialization.NodeConfig) *initialization.NodeConfig {
		return &initialization.NodeConfig{
			Name:               fmt.Sprintf("%s-%s", cfg.Name, identifier),
			Pruning:            cfg.Pruning,
			PruningKeepRecent:  cfg.PruningKeepRecent,
			PruningInterval:    cfg.PruningInterval,
			SnapshotInterval:   cfg.SnapshotInterval,
			SnapshotKeepRecent: cfg.SnapshotKeepRecent,
			IsValidator:        cfg.IsValidator,
			BtcNetwork:         cfg.BtcNetwork,
		}
	})
}

func updateNodeConfigs(cfgs []*initialization.NodeConfig, f func(cfg *initialization.NodeConfig) *initialization.NodeConfig) []*initialization.NodeConfig {
	result := make([]*initialization.NodeConfig, len(cfgs))
	for i, cfg := range cfgs {
		result[i] = f(cfg)
	}
	return result
}
