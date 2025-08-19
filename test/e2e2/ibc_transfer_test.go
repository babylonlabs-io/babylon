package e2e2

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2e2/types"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

	// Test IBC transfer from BSN to BBN
	t.Log("Testing IBC transfer from BSN to BBN...")

	bsnSender := bsn.DefaultWallet()
	bsnSender.VerifySentTx = true

	channelID := bsnChannels.Channels[0].ChannelId
	bbnRecipient := datagen.GenRandomAddress().String()
	transferAmount := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000000))

	txHash := bsn.SendIBCTransfer(bsnSender, bbnRecipient, transferAmount, channelID, "test transfer")
	require.NotEmpty(t, txHash, "Transaction hash should not be empty")

	t.Logf("IBC transfer submitted successfully with hash: %s", txHash)
}
