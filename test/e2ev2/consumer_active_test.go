package e2e2

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/stretchr/testify/require"
)

func TestConsumerActive(t *testing.T) {
	t.Parallel()
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

	consumers := bbn.QueryBTCStkConsumerConsumers()
	require.NotEmpty(t, consumers, "No consumers registered")

	consumerID := consumers[0].ConsumerId
	require.NotEmpty(t, consumerID, "Consumer ID should not be empty")

	resp := bbn.QueryConsumerActive(consumerID)
	require.NotNil(t, resp, "Response should not be nil")
	t.Logf("Consumer %s active status: %v", consumerID, resp.Active)

	t.Log("Consumer active test completed")
}
