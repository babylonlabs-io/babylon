package e2e2

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2e2/types"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	"github.com/stretchr/testify/require"
)

func TestIBCTransfer(t *testing.T) {
	t.Parallel()
	tm := types.NewTmWithIbc(t)
	tm.Start()

	bbn, bsn := tm.ChainBBN().Nodes[0], tm.ChainBSN().Nodes[0]

	t.Log("Verifying IBC channels were created...")
	bbnChannels := bbn.QueryIBCChannels()
	require.Len(t, bbnChannels.Channels, 1, "No IBC channels found on Babylon chain")
	bsnChannels := bsn.QueryIBCChannels()
	require.Len(t, bsnChannels.Channels, 1, "No IBC channels found on BSN Consumer chain")

	t.Log("Testing IBC transfer from BSN to BBN...")

	bsnSender := bsn.DefaultWallet()
	bsnSender.VerifySentTx = true

	channel := bsnChannels.Channels[0]
	channelID := channel.ChannelId
	bbnRecipient := datagen.GenRandomAddress().String()
	transferAmount := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000000))

	bsnSenderBalancesBefore := bsn.QueryAllBalances(bsnSender.Addr())
	bbnRecipientBalancesBefore := bbn.QueryAllBalances(bbnRecipient)
	t.Logf("Before transfer - Sender balance: %s, Recipient balance: %s", bsnSenderBalancesBefore.String(), bbnRecipientBalancesBefore.String())

	ibcTxHash := bsn.SendIBCTransfer(bsnSender, bbnRecipient, transferAmount, channelID, "test transfer")
	t.Logf("IBC transfer submitted successfully with tx hash: %s", ibcTxHash)

	// Compute the expected IBC denom on Babylon side
	// When tokens are transferred from BSN to BBN, the denom gets prefixed with transfer/channel-X and latter hashed to ibc/
	hop := transfertypes.NewHop(channel.Counterparty.PortId, channel.Counterparty.ChannelId)
	denomTrace := transfertypes.NewDenom(transferAmount.Denom, hop)
	expectedIBCDenom := denomTrace.IBCDenom()
	t.Logf("Expected IBC denom on Babylon: %s", expectedIBCDenom)

	// Wait for IBC transfer to complete and verify balance changes on both sides
	require.Eventually(t, func() bool {
		bbnRecipientBalancesAfter := bbn.QueryAllBalances(bbnRecipient)
		expAfterBalances := bbnRecipientBalancesBefore.Add(sdk.NewCoin(expectedIBCDenom, transferAmount.Amount))

		return bbnRecipientBalancesAfter.Equal(expAfterBalances)
	}, 30*time.Second, 2*time.Second, "IBC transfer should complete within 30 seconds")

	bsnSenderBalancesAfter := bsn.QueryAllBalances(bsnSender.Addr())
	ibcTxResp := bsn.QueryTxByHash(ibcTxHash)

	expBsnSendBalances := bsnSenderBalancesBefore.Sub(ibcTxResp.Tx.GetFee()...).Sub(transferAmount)
	require.Equal(t, expBsnSendBalances.String(), bsnSenderBalancesAfter.String(), "Sender should have %s, but has %s", expBsnSendBalances.String(), bsnSenderBalancesAfter.String())
}
