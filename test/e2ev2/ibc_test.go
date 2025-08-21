package e2e2

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	"github.com/stretchr/testify/require"
)

func TestIBCTransfer(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, bsn := tm.ChainNodes()

	t.Log("Testing IBC transfer from BSN to BBN...")
	bsnSender := bsn.DefaultWallet()
	bsnSender.VerifySentTx = true

	bsnChannels := bsn.QueryIBCChannels()
	bsnChannel := bsnChannels.Channels[0]
	bbnRecipient := datagen.GenRandomAddress().String()
	ibcTransferCoin := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000000))

	bsnSenderBalancesBefore := bsn.QueryAllBalances(bsnSender.Addr())
	bbnRecipientBalancesBefore := bbn.QueryAllBalances(bbnRecipient)
	t.Logf("Before transfer - Sender balance: %s, Recipient balance: %s", bsnSenderBalancesBefore.String(), bbnRecipientBalancesBefore.String())

	ibcTxHash := bsn.SendIBCTransfer(bsnSender, bbnRecipient, ibcTransferCoin, bsnChannel.ChannelId, "test transfer")
	t.Logf("IBC transfer submitted successfully with tx hash: %s", ibcTxHash)

	// Compute the expected IBC denom on Babylon side
	// When tokens are transferred from BSN to BBN, the denom gets prefixed with transfer/channel-X and latter hashed to ibc/
	hop := transfertypes.NewHop(bsnChannel.Counterparty.PortId, bsnChannel.Counterparty.ChannelId)
	denomTrace := transfertypes.NewDenom(ibcTransferCoin.Denom, hop)
	expectedIBCDenom := denomTrace.IBCDenom()
	t.Logf("Expected IBC denom on Babylon: %s", expectedIBCDenom)

	// Wait for IBC transfer to complete and verify balance changes on both sides
	require.Eventually(t, func() bool {
		bbnRecipientBalancesAfter := bbn.QueryAllBalances(bbnRecipient)
		expAfterBalances := bbnRecipientBalancesBefore.Add(sdk.NewCoin(expectedIBCDenom, ibcTransferCoin.Amount))

		return bbnRecipientBalancesAfter.Equal(expAfterBalances)
	}, 30*time.Second, 2*time.Second, "IBC transfer should complete within 30 seconds")

	bsnSenderBalancesAfter := bsn.QueryAllBalances(bsnSender.Addr())
	ibcTxResp := bsn.QueryTxByHash(ibcTxHash)

	expBsnSendBalances := bsnSenderBalancesBefore.Sub(ibcTxResp.Tx.GetFee()...).Sub(ibcTransferCoin)
	require.Equal(t, expBsnSendBalances.String(), bsnSenderBalancesAfter.String(), "Sender should have %s, but has %s", expBsnSendBalances.String(), bsnSenderBalancesAfter.String())
}
