package configurer

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	govv1 "cosmossdk.io/api/cosmos/gov/v1"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/config"
	"github.com/babylonlabs-io/babylon/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
)

type UpgradeSettings struct {
	IsEnabled  bool
	Version    string
	ForkHeight int64 // non-zero height implies that this is a fork upgrade.
}

type UpgradeConfigurer struct {
	baseConfigurer
	upgradeJsonFilePath string
	forkHeight          int64 // forkHeight > 0 implies that this is a fork upgrade. Otherwise, proposal upgrade.
}

var _ Configurer = (*UpgradeConfigurer)(nil)

// NewUpgradeConfigurer returns a upgrade configurer, if forkHeight is bigger
// than 0 it implies that it is a fork upgrade that does not pass by a gov prop
// if it is set to zero it runs the upgrade by the gov prop.
func NewUpgradeConfigurer(t *testing.T, chainConfigs []*chain.Config, setupTests setupFn, containerManager *containers.Manager, upgradePlanFilePath string, forkHeight int64) Configurer {
	t.Helper()
	return &UpgradeConfigurer{
		baseConfigurer: baseConfigurer{
			chainConfigs:     chainConfigs,
			containerManager: containerManager,
			setupTests:       setupTests,
			syncUntilHeight:  forkHeight + defaultSyncUntilHeight,
			t:                t,
		},
		forkHeight:          forkHeight,
		upgradeJsonFilePath: upgradePlanFilePath,
	}
}

func (uc *UpgradeConfigurer) ConfigureChains() error {
	errCh := make(chan error, len(uc.chainConfigs))
	var wg sync.WaitGroup

	for _, chainConfig := range uc.chainConfigs {
		wg.Add(1)
		go func(cc *chain.Config) {
			defer wg.Done()
			if err := uc.ConfigureChain(cc); err != nil {
				errCh <- err
			}
		}(chainConfig)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errCh)

	// Check if any of the goroutines returned an error
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (uc *UpgradeConfigurer) ConfigureChain(chainConfig *chain.Config) error {
	uc.t.Logf("starting upgrade e2e infrastructure for chain-id: %s", chainConfig.Id)
	tmpDir, err := os.MkdirTemp("", "bbn-e2e-testnet-*")
	if err != nil {
		return err
	}

	validatorConfigBytes, err := json.Marshal(chainConfig.ValidatorInitConfigs)
	if err != nil {
		return err
	}

	forkHeight := uc.forkHeight
	if forkHeight > 0 {
		forkHeight = forkHeight - config.ForkHeightPreUpgradeOffset
	}

	chainInitResource, err := uc.containerManager.RunChainInitResource(chainConfig.Id, int(chainConfig.VotingPeriod), int(chainConfig.ExpeditedVotingPeriod), validatorConfigBytes, tmpDir, int(forkHeight))
	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("%v/%v-encode", tmpDir, chainConfig.Id)
	uc.t.Logf("serialized init file for chain-id %v: %v", chainConfig.Id, fileName)

	// loop through the reading and unmarshaling of the init file a total of maxRetries or until error is nil
	// without this, test attempts to unmarshal file before docker container is finished writing
	var initializedChain initialization.Chain
	for i := 0; i < config.MaxRetries; i++ {
		initializedChainBytes, _ := os.ReadFile(fileName)
		err = json.Unmarshal(initializedChainBytes, &initializedChain)
		if err == nil {
			break
		}

		if i == config.MaxRetries-1 {
			return err
		}

		if i > 0 {
			time.Sleep(1 * time.Second)
		}
	}
	if err := uc.containerManager.PurgeResource(chainInitResource); err != nil {
		return err
	}
	uc.initializeChainConfigFromInitChain(&initializedChain, chainConfig)
	return nil
}

func (uc *UpgradeConfigurer) CreatePreUpgradeState() error {
	// send a few bank transfers simulating state data
	amountToSend := sdk.NewCoin(appparams.BaseCoinUnit, sdkmath.NewInt(1000000)) // 1bbn
	for _, chain := range uc.chainConfigs {
		firstNode := chain.NodeConfigs[0]
		otherNodes := chain.NodeConfigs[1:]
		// first node send to others...

		addresses := make([]string, len(otherNodes))
		for i, node := range otherNodes {
			addresses[i] = node.PublicAddress
		}
		firstNode.BankMultiSendFromNode(addresses, amountToSend.String())
	}

	return nil
}

func (uc *UpgradeConfigurer) RunSetup() error {
	return uc.setupTests(uc)
}

func (uc *UpgradeConfigurer) RunUpgrade() error {
	var err error
	if uc.forkHeight > 0 {
		uc.runForkUpgrade()
	} else {
		err = uc.runProposalUpgrade()
	}
	if err != nil {
		return err
	}

	// Check if the nodes are running
	for chainIndex, chainConfig := range uc.chainConfigs {
		chain := uc.baseConfigurer.GetChainConfig(chainIndex)
		for validatorIdx := range chainConfig.NodeConfigs {
			node := chain.NodeConfigs[validatorIdx]
			// Check node status
			_, err = node.Status()
			if err != nil {
				uc.t.Errorf("node is not running after upgrade, chain-id %s, node %s", chainConfig.Id, node.Name)
				return err
			}
			uc.t.Logf("node %s upgraded successfully, address %s", node.Name, node.PublicAddress)
		}
	}
	return nil
}

func (uc *UpgradeConfigurer) runProposalUpgrade() error {
	// submit, deposit, and vote for upgrade proposal
	// prop height = current height + voting period + time it takes to submit proposal + small buffer
	for _, chainConfig := range uc.chainConfigs {
		node, err := chainConfig.GetDefaultNode()
		if err != nil {
			return err
		}
		// currentHeight, err := node.QueryCurrentHeight()
		// if err != nil {
		// 	return err
		// }
		// TODO: need to make a way to update proposal height
		// chainConfig.UpgradePropHeight = currentHeight + int64(chainConfig.VotingPeriod) + int64(config.PropSubmitBlocks) + int64(config.PropBufferBlocks)
		chainConfig.UpgradePropHeight = 25 // at least read from the prop plan file
		propID := node.TxGovPropSubmitProposal(uc.upgradeJsonFilePath, node.WalletName)

		chainConfig.TxGovVoteFromAllNodes(propID, govv1.VoteOption_VOTE_OPTION_YES)
	}

	// wait till all chains halt at upgrade height
	for _, chainConfig := range uc.chainConfigs {
		uc.t.Logf("waiting to reach upgrade height on chain %s", chainConfig.Id)
		chainConfig.WaitUntilHeight(chainConfig.UpgradePropHeight)
		uc.t.Logf("upgrade height %d reached on chain %s", chainConfig.UpgradePropHeight, chainConfig.Id)
	}

	// remove all containers so we can upgrade them to the new version
	for _, chainConfig := range uc.chainConfigs {
		for _, validatorConfig := range chainConfig.NodeConfigs {
			err := uc.containerManager.RemoveNodeResource(validatorConfig.Name)
			if err != nil {
				return err
			}
		}
	}

	// remove all containers so we can upgrade them to the new version
	for _, chainConfig := range uc.chainConfigs {
		if err := uc.upgradeContainers(chainConfig, chainConfig.UpgradePropHeight); err != nil {
			return err
		}
	}
	return nil
}

func (uc *UpgradeConfigurer) runForkUpgrade() {
	for _, chainConfig := range uc.chainConfigs {
		uc.t.Logf("waiting to reach fork height on chain %s", chainConfig.Id)
		chainConfig.WaitUntilHeight(uc.forkHeight)
		uc.t.Logf("fork height reached on chain %s", chainConfig.Id)
	}
}

func (uc *UpgradeConfigurer) upgradeContainers(chainConfig *chain.Config, propHeight int64) error {
	// upgrade containers to the locally compiled daemon
	uc.t.Logf("starting upgrade for chain-id: %s...", chainConfig.Id)
	uc.containerManager.CurrentRepository = containers.BabylonContainerName

	errCh := make(chan error, len(chainConfig.NodeConfigs))
	var wg sync.WaitGroup

	for _, node := range chainConfig.NodeConfigs {
		wg.Add(1)
		go func(node *chain.NodeConfig) {
			defer wg.Done()
			if err := node.Run(); err != nil {
				errCh <- err
			}
		}(node)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errCh)

	// Check if any of the goroutines returned an error
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	uc.t.Logf("waiting to upgrade containers on chain %s", chainConfig.Id)
	chainConfig.WaitUntilHeight(propHeight + 1)
	uc.t.Logf("upgrade successful on chain %s", chainConfig.Id)
	return nil
}
