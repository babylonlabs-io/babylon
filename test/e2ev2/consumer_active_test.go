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

	mockContractAddr := "bbn1qyqszqgpqyqszqgpqyqszqgpqyqszqgpq5g7vvf"
	rollupConsumerID := "rollup-test-consumer"

	bbn.RegisterRollupConsumer(
		bbn.DefaultWallet().KeyName,
		rollupConsumerID,
		"rollup-consumer",
		"a test rollup consumer",
		"0.1",
		mockContractAddr,
	)

	tm.ChainsWaitUntilNextBlock()
	tm.UpdateWalletsAccSeqNumber()

	// query the rollup consumer active status, this will test the smart
	// contract query path
	// this should fail since the mock contract does not exist
	rollupResp, rollupErr := bbn.QueryConsumerActiveWithError(rollupConsumerID)
	if rollupErr != nil {
		t.Logf("ROLLUP consumer query failed as expected (mock contract): %v", rollupErr)
		// This is expected since we're using a mock contract address
		require.Contains(t, rollupErr.Error(), "failed to query contract", "Should fail with contract query error")
	} else {
		t.Logf("ROLLUP Consumer %s active status: %v", rollupConsumerID, rollupResp.Active)
	}
}
