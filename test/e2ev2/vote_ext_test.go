package e2e2

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
)

func TestCheckpointingDos_TwoValidators(t *testing.T) {
	t.Parallel()

	tm := tmanager.NewTestManager(t)

	// Single Babylon chain, but with TWO validators instead of the default one.
	bbnCfg := tmanager.NewChainConfig(tm.TempDir, tmanager.CHAIN_ID_BABYLON)
	bbnCfg.ValidatorCount = 3
	bbnCfg.NodeCount = 0

	// only updates the image of the bad one
	bbnChain := tmanager.NewChain(tm, bbnCfg)
	bbnChain.Validators[0].Container.Repository = "babylonlabs-io/babylond-bad"

	tm.Chains[tmanager.CHAIN_ID_BABYLON] = bbnChain

	// Start all nodes (3 validators) in Docker.
	tm.Start()

	// Let the chain produce some blocks so multiple proposals / checkpoints happen.
	const targetHeight = 30
	tm.ChainsWaitUntilHeight(targetHeight)

}
