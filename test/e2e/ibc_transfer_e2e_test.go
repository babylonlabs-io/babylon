package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v2/test/e2e/configurer"
	sdk "github.com/cosmos/cosmos-sdk/types"

	pfmroutertypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	"github.com/stretchr/testify/suite"
)

type IBCTransferTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
	addrA      string
	addrB      string
}

const nativeDenom = "ubbn"

func (s *IBCTransferTestSuite) SetupSuite() {
	s.T().Log("setting up IBC test suite...")
	var (
		err error
	)

	s.configurer, err = configurer.NewIBCTransferConfigurer(s.T(), true)

	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *IBCTransferTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func getFirstIBCDenom(balance sdk.Coins) string {
	// Look up the ugly IBC denom
	denoms := balance.Denoms()
	var denomB string
	for _, d := range denoms {
		if strings.HasPrefix(d, "ibc/") {
			denomB = d
			break
		}
	}
	return denomB
}

func (s *IBCTransferTestSuite) Test1IBCTransfer() {
	amount := int64(100_000)

	transferCoin := sdk.NewInt64Coin(nativeDenom, amount)

	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(2)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	// Check balance of val in chain-A (Node 3)
	s.addrA = nA.KeysAdd("addr-A")
	nA.BankSendFromNode(s.addrA, "10000000ubbn")

	s.addrB = nB.KeysAdd("addr-B")
	nB.BankSendFromNode(s.addrB, "10000000ubbn")

	nB.WaitForNextBlock()
	nA.WaitForNextBlock()

	balanceBeforeSendAddrA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)
	// Confirm val on A has enough funds
	s.Assert().GreaterOrEqual(balanceBeforeSendAddrA.AmountOf(nativeDenom).Int64(), amount)

	balanceBeforeSendAddrB, err := nB.QueryBalances(s.addrB)
	s.Require().NoError(err)
	// Only one denom in B
	s.Require().Len(balanceBeforeSendAddrB, 1)

	// Send transfer from val in chain-A (Node 3) to val in chain-B (Node 3)
	txHash := nA.SendIBCTransfer(s.addrA, s.addrB, "transfer", transferCoin)
	nA.WaitForNextBlock()

	_, txResp := nA.QueryTx(txHash)
	txFeesPaid := txResp.AuthInfo.Fee.Amount
	s.Require().Eventually(func() bool {
		// Check that the transfer is successful.
		// Amounts have been discounted from val in chain-A and added (as a wrapped denom) to val in chain-B
		balanceAfterSendAddrA, err := nA.QueryBalances(s.addrA)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}

		expectedAmt := balanceBeforeSendAddrA.Sub(transferCoin).Sub(txFeesPaid...).String()
		actualAmt := balanceAfterSendAddrA.String()

		if !strings.EqualFold(expectedAmt, actualAmt) {
			s.T().Logf(
				"BalanceBeforeSendAddrA: %s; BalanceAfterSendAddrA: %s, txFees: %s, coinTransfer: %s",
				balanceBeforeSendAddrA.String(), balanceAfterSendAddrA.String(), txFeesPaid.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer was not successful")

	s.Require().Eventually(func() bool {
		balanceAfterSendAddrB, err := nB.QueryBalances(s.addrB)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}
		// Check that there are now two denoms in B
		if len(balanceAfterSendAddrB) != 2 {
			return false
		}

		denomB := getFirstIBCDenom(balanceAfterSendAddrB)
		if !balanceAfterSendAddrB.AmountOf(denomB).Equal(transferCoin.Amount) {
			s.T().Logf(
				"BalanceBeforeSendAddrB: %s; BalanceBeforeSendAddrB: %s, txFees: %s, coinTransfer: %s",
				balanceBeforeSendAddrB.String(), balanceAfterSendAddrB.String(), txFeesPaid.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer was not successful")
}

func (s *IBCTransferTestSuite) Test2IBCTransferBack() {
	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(0)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	balanceBeforeSendBackB, err := nB.QueryBalances(s.addrB)
	s.Require().NoError(err)
	// Two denoms in B
	s.Require().Len(balanceBeforeSendBackB, 2)
	// Look for the ugly IBC one
	denom := getFirstIBCDenom(balanceBeforeSendBackB)
	amount := balanceBeforeSendBackB.AmountOf(denom).Int64() // have to pay gas fees

	transferCoin := sdk.NewInt64Coin(denom, amount)

	// Send transfer from val in chain-B (Node 3) to val in chain-A (Node 1)
	balanceBeforeReceivingSendBackA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)

	txHash := nB.SendIBCTransfer(s.addrB, s.addrA, "transfer back", transferCoin)

	nB.WaitForNextBlock()

	_, txResp := nB.QueryTx(txHash)
	txFeesPaid := txResp.AuthInfo.Fee.Amount

	s.Require().Eventually(func() bool {
		balanceAfterSendBackB, err := nB.QueryBalances(s.addrB)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}
		expectedAmt := balanceBeforeSendBackB.Sub(transferCoin).Sub(txFeesPaid...).String()
		actualAmt := balanceAfterSendBackB.String()

		if !strings.EqualFold(expectedAmt, actualAmt) {
			s.T().Logf(
				"BalanceBeforeSendBackB: %s; BalanceAfterSendBackB: %s, txFees: %s, coinTransfer: %s",
				balanceBeforeSendBackB.String(), balanceAfterSendBackB.String(), txFeesPaid.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer back A was not successful")

	nativeCoin := sdk.NewInt64Coin(nativeDenom, amount)
	s.Require().Eventually(func() bool {
		balanceAfterReceivingSendBackA, err := nA.QueryBalances(s.addrA)
		if err != nil {
			return false
		}
		// Check that there's still one denom in A
		if len(balanceAfterReceivingSendBackA) != 1 {
			return false
		}

		expectedAmt := balanceBeforeReceivingSendBackA.Add(nativeCoin).String()
		actualAmt := balanceAfterReceivingSendBackA.String()

		// Check that the balance of the native denom has increased
		if !strings.EqualFold(expectedAmt, actualAmt) {
			s.T().Logf(
				"BalanceBeforeReceivingSendBackA: %s; BalanceAfterReceivingSendBackA: %s, coinTransfer: %s",
				balanceBeforeReceivingSendBackA.String(), balanceAfterReceivingSendBackA.String(), nativeCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer back B was not successful")
}

// TestPacketForwarding sends a packet from chainB to chainA, and forwards it
// back to chainB
func (s *IBCTransferTestSuite) TestPacketForwarding() {
	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(0)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	balanceBeforeSendBackB, err := nB.QueryBalances(s.addrB)
	s.Require().NoError(err)

	transferCoin := sdk.NewInt64Coin(nativeDenom, 100_000)

	// Send transfer from val in chain-B (Node 3) to val in chain-A (Node 1)
	balanceBeforeReceivingSendBackA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)

	// Generate the forward metadata back to original sender in chain B
	forwardMetadata := pfmroutertypes.ForwardMetadata{
		Receiver: s.addrB,
		Port:     "transfer",
		Channel:  "channel-0",
	}
	memoData := pfmroutertypes.PacketMetadata{Forward: &forwardMetadata}
	forwardMemo, err := json.Marshal(memoData)
	s.NoError(err)

	txHash := nB.SendIBCTransfer(s.addrB, s.addrA, string(forwardMemo), transferCoin)

	nB.WaitForNextBlock()

	txRes, tx, err := nB.QueryTxWithError(txHash)
	s.Require().NoError(err)
	// check tx was successful
	s.Require().Zero(txRes.Code, fmt.Sprintf("Tx response with non-zero code. Code: %d - Raw log: %s", txRes.Code, txRes.RawLog))
	txFeesPaid := tx.AuthInfo.Fee.Amount

	s.Require().Eventually(func() bool {
		balanceAfterSendBackB, err := nB.QueryBalances(s.addrB)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}
		// expected to have the same initial balance - fees
		// because the pkg with funds makes a round trip
		expectedAmt := balanceBeforeSendBackB.Sub(txFeesPaid...).String()
		actualAmt := balanceAfterSendBackB.String()

		if !strings.EqualFold(expectedAmt, actualAmt) {
			s.T().Logf(
				"BalanceBeforeSendBackB: %s; BalanceAfterSendBackB: %s, txFees: %s, coinTransfer: %s",
				balanceBeforeSendBackB.String(), balanceAfterSendBackB.String(), txFeesPaid.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer back A was not successful")

	s.Require().Eventually(func() bool {
		balanceAfterReceivingSendBackA, err := nA.QueryBalances(s.addrA)
		if err != nil {
			return false
		}

		// balance should remain unchanged on chain A
		expectedAmt := balanceBeforeReceivingSendBackA.String()
		actualAmt := balanceAfterReceivingSendBackA.String()

		// Check that the balance of the native denom has increased
		if !strings.EqualFold(expectedAmt, actualAmt) {
			s.T().Logf(
				"BalanceBeforeReceivingSendBackA: %s; BalanceAfterReceivingSendBackA: %s",
				balanceBeforeReceivingSendBackA.String(), balanceAfterReceivingSendBackA.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer back B was not successful")
}

func (s *IBCTransferTestSuite) Test4MultiCoinFee() {
	amount := int64(1_000)

	transferCoin := sdk.NewInt64Coin(nativeDenom, amount)

	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(2)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	// Check balance
	balanceBeforeSendAddrB, err := nB.QueryBalances(s.addrB)
	s.Require().NoError(err)

	// Send transfer from val in chain-A (Node 3) to val in chain-B (Node 3)
	nA.SendIBCTransfer(s.addrA, s.addrB, "transfer", transferCoin)
	nA.WaitForNextBlock()

	var ibcDenomB string
	s.Require().Eventually(func() bool {
		balanceAfterSendAddrB, err := nB.QueryBalances(s.addrB)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}
		// Check that there are now two denoms in B
		if len(balanceAfterSendAddrB) != 2 {
			return false
		}

		ibcDenomB = getFirstIBCDenom(balanceAfterSendAddrB)
		if ibcDenomB == "" {
			return false
		}
		expAmt := balanceBeforeSendAddrB.AmountOf(ibcDenomB).Add(transferCoin.Amount)
		if !balanceAfterSendAddrB.AmountOf(ibcDenomB).Equal(expAmt) {
			s.T().Logf(
				"BalanceBeforeSendAddrB: %s; BbalanceAfterSendAddrB: %s, coinTransfer: %s",
				balanceBeforeSendAddrB.String(), balanceAfterSendAddrB.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer was not successful")

	// Send some funds to new address
	// using as fees other denom than the native denom
	to := nB.KeysAdd("new-addr")
	feesStr := fmt.Sprintf("%d%s,%d%s", 400, nativeDenom, 1, ibcDenomB)
	nB.LogActionF("bank sending %s from wallet %s to %s. Fees: %s", transferCoin, s.addrB, to, feesStr)
	cmd := []string{
		"babylond", "tx", "bank", "send",
		s.addrB, to, transferCoin.String(),
		fmt.Sprintf("--from=%s", s.addrB),
		fmt.Sprintf("--fees=%s", feesStr),
		fmt.Sprintf("--chain-id=%s", nB.GetChainID()),
		"--yes",
		"--keyring-backend=test", "--log_format=json", "--home=/home/babylon/babylondata",
	}

	// Tx should fail
	outBuf, _, err := nB.ExecRawCmd(cmd)
	s.Require().NoError(err)
	s.Require().Contains(outBuf.String(), fmt.Sprintf("can only receive bond\n  denom %s", nativeDenom))
	nA.WaitForNextBlock()

	// Try to send funds to fee_collector
	balanceBeforeAddrA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)

	feeCollectorAddr := "bbn17xpfvakm2amg962yls6f84z3kell8c5l88j35y"
	txHash := nA.SendIBCTransfer(s.addrA, feeCollectorAddr, "transfer", transferCoin)
	nA.WaitForNextBlock()

	_, txResp := nA.QueryTx(txHash)
	txFeesPaid := txResp.AuthInfo.Fee.Amount
	// Make sure only fees were deducted from sender
	// The tx should have failed
	s.Require().Eventually(func() bool {
		balanceAfterAddrA, err := nA.QueryBalances(s.addrA)
		s.Require().NoError(err)
		return balanceAfterAddrA.Equal(balanceBeforeAddrA.Sub(txFeesPaid...))
	}, 90*time.Second, 2*time.Second)
}

func (s *IBCTransferTestSuite) Test5E2EBelowThreshold() {
	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(0)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	_, err = nB.QueryBalances(s.addrB)
	s.Require().NoError(err)

	transferCoin := sdk.NewInt64Coin(nativeDenom, 100)

	balanceBeforeReceivingSendA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)

	txHash := nB.SendIBCTransfer(s.addrB, s.addrA, "channel-0", transferCoin)

	nB.WaitForNextBlock()

	txRes, _, err := nB.QueryTxWithError(txHash)
	s.Require().NoError(err)
	s.Require().Zero(txRes.Code, fmt.Sprintf("Tx response with non-zero code. Code: %d - Raw log: %s", txRes.Code, txRes.RawLog))

	s.Require().Eventually(func() bool {
		balanceAfterReceivingSendA, err := nA.QueryBalances(s.addrA)
		if err != nil {
			return false
		}

		before := balanceBeforeReceivingSendA.String()
		after := balanceAfterReceivingSendA.String()

		s.Require().NotEqual(before, after)

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer back B was not successful")
}

func (s *IBCTransferTestSuite) Test6RateLimitE2EAboveThreshold() {
	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(0)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	balanceBeforeTransferA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)
	s.T().Logf("Balance before transfer for addrA: %s", balanceBeforeTransferA.String())

	_, err = nB.QueryBalances(s.addrB)
	s.Require().NoError(err)

	packetAmount := sdkmath.NewInt(1_000_001) // above the threshold and should fail
	channel := "channel-0"

	transferCoin := sdk.NewCoin(nativeDenom, packetAmount)

	s.T().Log("Attempting to send IBC transfer...")
	txHash := nB.SendIBCTransfer(s.addrB, s.addrA, channel, transferCoin)
	nB.WaitForNextBlock()

	txRes, _, err := nB.QueryTxWithError(txHash)
	s.Require().NoError(err)
	s.Require().NotZero(txRes.Code, fmt.Sprintf("Tx was suppossed to fail. Code: %d", txRes.Code))
	s.Require().Contains(txRes.RawLog, "quota exceeded")

	if txHash != "" {
		s.T().Logf("IBC transfer sent, txHash: %s", txHash)
	}

	balanceAfterReceivingSendBackA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)

	s.Require().Equal(balanceBeforeTransferA.String(), balanceAfterReceivingSendBackA.String(), "Balance should remain unchanged after failed transfer (only paid for fees)")
}
