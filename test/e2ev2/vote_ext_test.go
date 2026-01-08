package e2e2

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
)

const (
	badBbnImage = "babylonlabs-io/babylond-bad"
)

func TestBigVoteExtDup(t *testing.T) {
	t.Parallel()

	tm := tmanager.NewTestManager(t)

	// Single Babylon chain, but with TWO validators instead of the default one.
	bbnCfg := tmanager.NewChainConfig(tm.TempDir, tmanager.CHAIN_ID_BABYLON)
	bbnCfg.ValidatorCount = 12
	bbnCfg.NodeCount = 0

	// only updates the image of the bad one
	bbnChain := tmanager.NewChain(tm, bbnCfg)
	bbnChain.Validators[0].Container.Repository = badBbnImage
	bbnChain.Validators[1].Container.Repository = badBbnImage
	bbnChain.Validators[2].Container.Repository = badBbnImage

	tm.Chains[tmanager.CHAIN_ID_BABYLON] = bbnChain

	// Start all nodes (12 validators) in Docker.
	tm.Start()

	// Let the chain produce some blocks so multiple proposals / checkpoints happen.
	const targetHeight = 150
	tm.ChainsWaitUntilHeight(targetHeight)

}
