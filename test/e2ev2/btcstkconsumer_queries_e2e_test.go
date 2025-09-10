package e2e2

import (
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	zoneconciergetype "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
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

	bbnChannels := bbn.QueryIBCChannels()
	require.NotEmpty(t, bbnChannels.Channels)
	connectionID := bbnChannels.Channels[0].ConnectionHops[0]

	err := tm.Hermes.CreateZoneConciergeChannel(tm.ChainBBN(), tm.ChainBSN(),
		connectionID)
	require.NoError(t, err, "failed to create zoneconcierge channel")

	bbn.WaitForCondition(func() bool {
		chans := bbn.QueryIBCChannels()
		return len(chans.Channels) == 2 && chans.Channels[1].PortId == zoneconciergetype.PortID && chans.Channels[1].State == channeltypes.OPEN
	}, "timed out waiting for zoneconcierge channel to open")

	consumers := bbn.QueryBTCStkConsumerConsumers()
	require.NotEmpty(t, consumers, "no consumers registered")

	consumerID := consumers[0].ConsumerId
	require.NotEmpty(t, consumerID, "consumer ID should not be empty")

	resp, err := bbn.QueryConsumerActive(consumerID)
	require.NoError(t, err)
	require.NotNil(t, resp, "response should not be nil")
	require.True(t, resp, "cosmos consumer should be active")

	tm.UpdateWalletsAccSeqNumber()

	contractAddr := bbn.CreateFinalityContract("rollup-test-consumer")

	bbn.RegisterRollupConsumer(
		bbn.DefaultWallet().KeyName,
		"rollup-test-consumer",
		"rollup-consumer",
		"a test rollup consumer",
		"0.1",
		contractAddr,
	)

	tm.ChainsWaitUntilNextBlock()
	tm.UpdateWalletsAccSeqNumber()

	require.Eventually(t, func() bool {
		consumers := bbn.QueryBTCStkConsumerConsumers()
		return len(consumers) >= 2
	}, time.Second*10, time.Millisecond*500, "Expected 2 consumers after rollup registration")

	rollupResp, err := bbn.QueryConsumerActive("rollup-test-consumer")
	require.NoError(t, err)
	require.NotNil(t, rollupResp, "rollup consumer resp should not be nil")
	require.True(t, rollupResp, "rollup should be active with real finality contract")

	validButNonExistentAddr := "cosmos1abc123def456ghi789jkl012mno345pqr678st"

	bbn.RegisterRollupConsumer(
		bbn.DefaultWallet().KeyName,
		"rollup-nonexistent-contract",
		"rollup-nonexistent",
		"a test rollup consumer with non-existent contract",
		"0.1",
		validButNonExistentAddr,
	)

	rollupNonExistentResp, err := bbn.QueryConsumerActive("rollup-nonexistent-contract")
	require.NoError(t, err)
	require.False(t, rollupNonExistentResp, "rollup with non-existent contract should be inactive")
}
