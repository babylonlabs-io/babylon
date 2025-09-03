package e2e2

import (
	"math/rand"
	"testing"
	"time"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	zoneconciergetype "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/stretchr/testify/require"
)

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

func TestZoneConciergeQueries(t *testing.T) {
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbnChain := tm.ChainBBN()
	bsnChain := tm.ChainBSN()
	bbn, bsn := bbnChain.Nodes[0], bsnChain.Nodes[0]

	bbn.RegisterConsumerChain(
		bbn.DefaultWallet().KeyName,
		"07-tendermint-0",
		"bsn-consumer",
		"a test consumer",
		"0.1",
	)

	bsn.RegisterConsumerChain(
		bsn.DefaultWallet().KeyName,
		"07-tendermint-0",
		"bsn-consumer",
		"a test consumer",
		"0.1",
	)

	tm.ChainsWaitUntilNextBlock()
	tm.UpdateWalletsAccSeqNumber()

	bbnChannels := bbn.QueryIBCChannels()
	require.NotEmpty(t, bbnChannels.Channels)
	connectionID := bbnChannels.Channels[0].ConnectionHops[0]

	err := tm.Hermes.CreateZoneConciergeChannel(tm.ChainBBN(), tm.ChainBSN(), connectionID)
	require.NoError(t, err, "failed to create zoneconcierge channel")

	bbn.WaitForCondition(func() bool {
		chans := bbn.QueryIBCChannels()
		return len(chans.Channels) == 2 && chans.Channels[1].PortId == zoneconciergetype.PortID && chans.Channels[1].State == channeltypes.OPEN
	}, "timed out waiting for zoneconcierge channel to open")

	tm.UpdateWalletsAccSeqNumber()
	headerWallet := bbn.DefaultWallet()
	headerWallet.VerifySentTx = true

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 3; i++ {
		bbn.InsertNewEmptyBtcHeader(r)
		tm.UpdateWalletsAccSeqNumber()
	}

	tm.ChainsWaitUntilNextBlock()

	consumers := bbn.QueryBTCStkConsumerConsumers()
	require.NotEmpty(t, consumers, "No consumers registered")

	consumerID := consumers[0].ConsumerId

	seg := bbn.QueryBSNLastSentSegment(consumerID)
	require.NotNil(t, seg,
		"expected non nil segment response even if no segment is available")
	require.Equal(t, 1, (len(seg.Segment.BtcHeaders)),
		"expected at most 1 header in initial segment")

	bbn.WaitForCondition(func() bool {
		proof := bbn.QueryGetSealedEpochProof(1)
		require.NotNil(t, proof.Epoch)
		return err == nil
	}, "timed out waiting for epoch 1 to be sealed")

	proofResp := bbn.QueryGetSealedEpochProof(1)
	require.NotNil(t, proofResp.Epoch)
	require.IsType(t, &zoneconciergetype.ProofEpochSealedResponse{},
		proofResp.Epoch)
}
