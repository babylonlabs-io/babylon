package configurer

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
)

type CurrentBranchConfigurer struct {
	baseConfigurer
}

var _ Configurer = (*CurrentBranchConfigurer)(nil)

func NewCurrentBranchConfigurer(t *testing.T, chainConfigs []*chain.Config, setupTests setupFn, containerManager *containers.Manager) Configurer {
	return &CurrentBranchConfigurer{
		baseConfigurer: baseConfigurer{
			chainConfigs:     chainConfigs,
			containerManager: containerManager,
			setupTests:       setupTests,
			syncUntilHeight:  defaultSyncUntilHeight,
			t:                t,
		},
	}
}

func (cb *CurrentBranchConfigurer) ConfigureChains() error {
	cb.t.Logf("Current branch configure chains")
	for _, chainConfig := range cb.chainConfigs {
		fmt.Printf("\nconfigure chain :%s - %s", chainConfig.Id, chainConfig.DataDir)
		if err := cb.ConfigureChain(chainConfig); err != nil {
			return err
		}
	}
	return nil
}

func (cb *CurrentBranchConfigurer) ConfigureChain(chainConfig *chain.Config) error {
	cb.t.Logf("starting e2e infrastructure from current branch for chain-id: %s", chainConfig.Id)
	tmpDir, err := os.MkdirTemp("", "bbn-e2e-testnet-*")
	if err != nil {
		return err
	}
	cb.t.Logf("temp directory for chain-id %v: %v", chainConfig.Id, tmpDir)
	fmt.Printf("\n tmp dir for config chain %s - %+v", tmpDir, chainConfig)
	initializedChain, err := initialization.InitChain(
		chainConfig.Id,
		tmpDir,
		chainConfig.ValidatorInitConfigs,
		time.Duration(chainConfig.VotingPeriod*1000000000),
		time.Duration(chainConfig.ExpeditedVotingPeriod*1000000000), 0)
	if err != nil {
		return err
	}
	cb.initializeChainConfigFromInitChain(initializedChain, chainConfig)
	return nil
}

func (cb *CurrentBranchConfigurer) RunSetup() error {
	return cb.setupTests(cb)
}
