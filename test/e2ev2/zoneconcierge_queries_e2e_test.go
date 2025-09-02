package e2e2

import (
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/stretchr/testify/require"
)

func TestZoneConciergeQueries(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, bsn := tm.ChainNodes()

	params := bbn.QueryZoneConciergeParams()
	require.NotNil(t, params)

	finalizedResp := bbn.QueryFinalizedBSNsInfo([]string{}, false)
	require.NotNil(t, finalizedResp)

	testConsumerID := bsn.ChainConfig.ChainID
	finalizedRespWithID := bbn.QueryFinalizedBSNsInfo([]string{testConsumerID}, true)
	require.NotNil(t, finalizedRespWithID)

	headerResp := bbn.QueryLatestEpochHeader(testConsumerID)
	require.NotNil(t, headerResp)

	segmentResp := bbn.QueryBSNLastSentSegment(testConsumerID)
	require.NotNil(t, segmentResp)

	epochNum := uint64(1)
	proofResp := bbn.QueryGetSealedEpochProof(epochNum)
	require.NotNil(t, proofResp)

	time.Sleep(5 * time.Second)

	proofResp0 := bbn.QueryGetSealedEpochProof(0)
	require.NotNil(t, proofResp0)

}

func TestZoneConciergeQueriesWithMultipleConsumers(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, bsn := tm.ChainNodes()

	testConsumerIDs := []string{bsn.ChainConfig.ChainID, "test-consumer-2", "test-consumer-3"}

	finalizedResp := bbn.QueryFinalizedBSNsInfo(testConsumerIDs, false)
	require.NotNil(t, finalizedResp)

	for _, consumerID := range testConsumerIDs {

		headerResp := bbn.QueryLatestEpochHeader(consumerID)
		require.NotNil(t, headerResp)

		segmentResp := bbn.QueryBSNLastSentSegment(consumerID)
		require.NotNil(t, segmentResp)

		time.Sleep(100 * time.Millisecond)
	}

}

func TestZoneConciergeQueriesCLI(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, bsn := tm.ChainNodes()
	testConsumerID := bsn.ChainConfig.ChainID

	headerRespCLI := bbn.QueryLatestEpochHeaderCLI(testConsumerID)
	require.NotNil(t, headerRespCLI)

	segmentRespCLI := bbn.QueryBSNLastSentSegmentCLI(testConsumerID)
	require.NotNil(t, segmentRespCLI)
	epochNum := uint64(1)
	proofRespCLI := bbn.QueryGetSealedEpochProofCLI(epochNum)
	require.NotNil(t, proofRespCLI)

	proofResp0CLI := bbn.QueryGetSealedEpochProofCLI(0)
	require.NotNil(t, proofResp0CLI)

}
