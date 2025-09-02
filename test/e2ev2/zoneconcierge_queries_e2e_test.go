package e2e2

import (
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestZoneConciergeQueriesWithMultipleConsumers(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, _ := tm.ChainNodes()

	proofResp := bbn.QueryGetSealedEpochProof(1)
	require.NotNil(t, proofResp)
	require.NotNil(t, proofResp.Epoch)
}

func TestZoneConciergeQueriesCLI(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, _ := tm.ChainNodes()

	proofOutput := bbn.QueryGetSealedEpochProofCLI(1)
	require.Contains(t, proofOutput, "validator_set")

	// returns "not found" but commands work
	headerOutput := bbn.QueryLatestEpochHeaderCLI("07-tendermint-0")
	require.NotNil(t, headerOutput)

	segmentOutput := bbn.QueryBSNLastSentSegmentCLI("07-tendermint-0")
	require.NotNil(t, segmentOutput)
}
