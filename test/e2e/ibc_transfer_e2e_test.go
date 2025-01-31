package e2e

import (
	"strings"
	"time"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
)

type IBCTransferTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
	addrA      string
	addrB      string
}

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
	denom := "ubbn"
	amount := int64(1_000_000)

	transferCoin := sdk.NewInt64Coin(denom, amount)

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
	s.Assert().GreaterOrEqual(balanceBeforeSendAddrA.AmountOf(denom).Int64(), amount)

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
	nativeDenom := "ubbn"

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
