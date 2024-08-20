package configurer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	zctypes "github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
)

type Configurer interface {
	ConfigureChains() error

	ClearResources() error

	GetChainConfig(chainIndex int) *chain.Config

	RunSetup() error

	RunValidators() error

	InstantiateBabylonContract() error

	RunHermesRelayerIBC() error

	// RunCosmosRelayerIBC configures IBC with Go relayer
	RunCosmosRelayerIBC() error

	RunIBCTransferChannel() error
}

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
		},
		{
			Name:               "babylon-default-a-2",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        true,
		},
		{
			Name:               "babylon-default-a-3",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        false,
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
		},
		{
			Name:               "babylon-default-b-2",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        true,
		},
		{
			Name:               "babylon-default-b-3",
			Pruning:            "nothing",
			PruningKeepRecent:  "0",
			PruningInterval:    "0",
			SnapshotInterval:   1500,
			SnapshotKeepRecent: 2,
			IsValidator:        false,
		},
	}
	ibcConfigChainA = &ibctesting.ChannelConfig{
		PortID:  zctypes.PortID,
		Order:   zctypes.Ordering,
		Version: zctypes.Version,
	}
	ibcConfigChainB = &ibctesting.ChannelConfig{
		PortID:  zctypes.PortID, // Will be replaced by the contract address in Phase 2 tests
		Order:   zctypes.Ordering,
		Version: zctypes.Version,
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
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), ibcConfigChainA),
			chain.New(t, containerManager, initialization.ChainBID, updateNodeConfigNameWithIdentifier(validatorConfigsChainB, identifier), ibcConfigChainB),
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
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), ibcConfigChainA),
			chain.New(t, containerManager, initialization.ChainBID, updateNodeConfigNameWithIdentifier(validatorConfigsChainB, identifier), ibcConfigChainB),
		},
		withIBCTransferChannel(baseSetup), // base set up with IBC
		containerManager,
	), nil
}

// NewBTCTimestampingPhase2Configurer returns a new Configurer for BTC timestamping service (phase 2).
func NewBTCTimestampingPhase2Configurer(t *testing.T, isDebugLogEnabled bool) (Configurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, false, false)
	if err != nil {
		return nil, err
	}

	return NewCurrentBranchConfigurer(t,
		[]*chain.Config{
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), ibcConfigChainA),
			chain.New(t, containerManager, initialization.ChainBID, updateNodeConfigNameWithIdentifier(validatorConfigsChainB, identifier), ibcConfigChainB),
		},
		withPhase2IBC(baseSetup), // IBC setup (requires contract address)
		containerManager,
	), nil
}

// NewBTCTimestampingPhase2RlyConfigurer returns a new Configurer for BTC timestamping service (phase 2), using the Go relayer (rly).
func NewBTCTimestampingPhase2RlyConfigurer(t *testing.T, isDebugLogEnabled bool) (Configurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, true, false)
	if err != nil {
		return nil, err
	}

	return NewCurrentBranchConfigurer(t,
		[]*chain.Config{
			chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), ibcConfigChainA),
			chain.New(t, containerManager, initialization.ChainBID, updateNodeConfigNameWithIdentifier(validatorConfigsChainB, identifier), ibcConfigChainB),
		},
		withPhase2RlyIBC(baseSetup), // IBC setup with wasmd and Go relayer
		containerManager,
	), nil
}

// NewBTCStakingConfigurer returns a new Configurer for BTC staking service
func NewBTCStakingConfigurer(t *testing.T, isDebugLogEnabled bool) (Configurer, error) {
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
func NewSoftwareUpgradeConfigurer(t *testing.T, isDebugLogEnabled bool, upgradePath string, btcHeaders []*btclighttypes.BTCHeaderInfo) (*UpgradeConfigurer, error) {
	identifier := identifierName(t)
	containerManager, err := containers.NewManager(identifier, isDebugLogEnabled, false, true)
	if err != nil {
		return nil, err
	}

	chainA := chain.New(t, containerManager, initialization.ChainAID, updateNodeConfigNameWithIdentifier(validatorConfigsChainA, identifier), nil)
	if btcHeaders != nil {
		chainA.BTCHeaders = btcHeaders
	}

	return NewUpgradeConfigurer(t,
		[]*chain.Config{
			// we only need 1 chain for testing upgrade
			chainA,
		},
		withUpgrade(baseSetup), // base set up with upgrade
		containerManager,
		upgradePath,
		0,
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
	result := make([]*initialization.NodeConfig, len(cfgs))
	for i, cfg := range cfgs {
		result[i] = &initialization.NodeConfig{
			Name:               fmt.Sprintf("%s-%s", cfg.Name, identifier),
			Pruning:            cfg.Pruning,
			PruningKeepRecent:  cfg.PruningKeepRecent,
			PruningInterval:    cfg.PruningInterval,
			SnapshotInterval:   cfg.SnapshotInterval,
			SnapshotKeepRecent: cfg.SnapshotKeepRecent,
			IsValidator:        cfg.IsValidator,
		}
	}
	return result
}
