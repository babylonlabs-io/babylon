package e2e2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/test/e2e2/types"
)

func TestBtcRewardsIbcCallback(t *testing.T) {
	t.Parallel()
	tm := types.NewTmWithIbc(t)
	tm.Start()

	// Setup chains and verify IBC channels
	t.Log("Verifying IBC channels were created...")
	bbn, bsn := tm.ChainBBN().Nodes[0], tm.ChainBSN().Nodes[0]
	bbnChannels := bbn.QueryIBCChannels()
	require.Len(t, bbnChannels.Channels, 1, "No IBC channels found on Babylon chain")

	bsnChannels := bsn.QueryIBCChannels()
	require.Len(t, bsnChannels.Channels, 1, "No IBC channels found on BSN Consumer chain")

	// 1. Create FP for babylon send signatures and start earning rewards, check rewards
	// 2. Create Consumer chains add btc delegations to it and bsn verify rewards
	// 3. Send Bsn rewards with IBC and verify
	// 4. Send ubbn from babylon to bbn and send as bsn rewards and verify the amounts
}
