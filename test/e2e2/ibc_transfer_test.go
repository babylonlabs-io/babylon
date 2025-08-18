package e2e2

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/test/e2e2/types"
	"github.com/stretchr/testify/require"
)

func TestIBCTransfer(t *testing.T) {
	tm := types.NewTmWithIbc(t)

	tm.Start()

	// Check that both IBC channels were created
	t.Log("Verifying IBC channels were created...")

	bbn := tm.ChainBBN().Nodes[0]
	bbnChannels := bbn.QueryIBCChannels()
	require.NotEmpty(t, bbnChannels.Channels, "No IBC channels found on Babylon chain")

	bsn := tm.ChainBSN().Nodes[0]
	bsnChannels := bsn.QueryIBCChannels()
	require.Len(t, bsnChannels.Channels, 1, "No IBC channels found on BSN Consumer chain")

	// Log channel information
	t.Logf("Babylon channels: %d", len(bbnChannels.Channels))
	for i, ch := range bbnChannels.Channels {
		t.Logf("  Channel %d: %s -> %s (State: %s, Port: %s)",
			i, ch.ChannelId, ch.Counterparty.ChannelId, ch.State, ch.PortId)
	}

	t.Logf("Consumer BSN channels: %d", len(bsnChannels.Channels))
	for i, ch := range bsnChannels.Channels {
		t.Logf("  Channel %d: %s -> %s (State: %s, Port: %s)",
			i, ch.ChannelId, ch.Counterparty.ChannelId, ch.State, ch.PortId)
	}

	// time.Sleep(100 * time.Second)
	// Verify tokens arrived on consumer chain
	// TODO: Implement balance queries

	// Send tokens back from Consumer to Babylon
	// TODO: Implement reverse transfer

	// Verify round-trip worked
	// TODO: Implement final balance verification

	// Test cleanup handled by t.Cleanup()
}
