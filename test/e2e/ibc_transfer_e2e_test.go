package e2e

import (
	"math"
	"strings"
	"time"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
)

type IBCTransferTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
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

	val := initialization.ValidatorWalletName

	// Check balance of val in chain-A (Node 3)
	addrA := nA.KeysAdd("addr-A")
	nA.BankSendFromNode(addrA, "10000000ubbn")

	addrB := nB.KeysAdd("addr-B")
	nB.BankSendFromNode(addrB, "10000000ubbn")

	nB.WaitForNextBlock()
	nA.WaitForNextBlock()

	balanceBeforeSendAddrA, err := nA.QueryBalances(addrA)
	s.Require().NoError(err)
	// Confirm val on A has enough funds
	s.Assert().GreaterOrEqual(balanceBeforeSendAddrA.AmountOf(denom).Int64(), amount)

	balanceBeforeSendAddrB, err := nB.QueryBalances(addrB)
	s.Require().NoError(err)
	// Only one denom in B
	s.Require().Len(balanceBeforeSendAddrB, 1)

	// Send transfer from val in chain-A (Node 3) to val in chain-B (Node 3)
	txHash := nA.SendIBCTransfer(val, addrB, "transfer", transferCoin)
	nA.WaitForNextBlock()

	_, txResp := nA.QueryTx(txHash)
	txFeesPaid := txResp.AuthInfo.Fee.Amount
	s.Require().Eventually(func() bool {
		// Check that the transfer is successful.
		// Amounts have been discounted from val in chain-A and added (as a wrapped denom) to val in chain-B
		balanceAfterSendAddrA, err := nA.QueryBalances(addrA)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}

		expectedAmt := balanceBeforeSendAddrA.Sub(transferCoin).Sub(txFeesPaid...).String()
		actualAmt := balanceAfterSendAddrA.String()

		if !strings.EqualFold(expectedAmt, actualAmt) {
			s.T().Logf(
				"BalanceBeforeSendAddrA: %s; BalanceBeforeSendAddrA: %s, txFees: %s, coinTransfer: %s",
				balanceBeforeSendAddrA.String(), balanceAfterSendAddrA.String(), txFeesPaid.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer was not successful")

	s.Require().Eventually(func() bool {
		balanceAfterSendAddrB, err := nB.QueryBalances(addrB)
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
	delta := float64(10000) // Tolerance to account for gas fees

	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	babylonNodeA, err := bbnChainA.GetNodeAtIndex(0)
	s.NoError(err)
	babylonNodeB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	val := initialization.ValidatorWalletName

	addrB := babylonNodeB.GetWallet(val)
	balanceB, err := babylonNodeB.QueryBalances(addrB)
	s.Require().NoError(err)
	// Two denoms in B
	s.Require().Len(balanceB, 2)
	// Look for the ugly IBC one
	denom := getFirstIBCDenom(balanceB)
	amount := balanceB.AmountOf(denom).Int64() - int64(delta) // have to pay gas fees

	transferCoin := sdk.NewInt64Coin(denom, amount)

	// Send transfer from val in chain-B (Node 3) to val in chain-A (Node 1)
	addrA := babylonNodeA.GetWallet(val)
	balanceA, err := babylonNodeA.QueryBalances(addrA)
	s.Require().NoError(err)

	babylonNodeB.SendIBCTransfer(val, addrA, "transfer back", transferCoin)

	s.Require().Eventually(func() bool {
		balanceB2, err := babylonNodeB.QueryBalances(addrB)
		if err != nil {
			return false
		}
		return math.Abs(float64(balanceB.Sub(transferCoin).AmountOf(denom).Int64()-
			balanceB2.AmountOf(denom).Int64())) < delta
	}, 1*time.Minute, 1*time.Second, "Transfer back A was not successful")

	nativeCoin := sdk.NewInt64Coin(nativeDenom, amount)
	s.Require().Eventually(func() bool {
		balanceA2, err := babylonNodeA.QueryBalances(addrA)
		if err != nil {
			return false
		}
		// Check that there's still one denom in A
		if len(balanceA2) != 1 {
			return false
		}
		// Check that the balance of the native denom has increased
		return math.Abs(float64(balanceA.Add(nativeCoin).AmountOf(nativeDenom).Int64()-
			balanceA2.AmountOf(nativeDenom).Int64())) < delta
	}, 1*time.Minute, 1*time.Second, "Transfer back B was not successful")
}
