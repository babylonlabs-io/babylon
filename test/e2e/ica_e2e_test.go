package e2e

import (
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"

	"github.com/stretchr/testify/suite"
)

const (
	icaConnectionID = "connection-0"
	icaChannel      = "channel-1"
)

type ICATestSuite struct {
	suite.Suite

	configurer configurer.Configurer
	addrA      string
}

func (s *ICATestSuite) SetupSuite() {
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

func (s *ICATestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *ICATestSuite) TestCreateInterchainAccount() {
	amount := int64(100_000)

	transferCoin := sdk.NewInt64Coin(nativeDenom, amount)

	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(2)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	// Send some funds to ICA owner
	s.addrA = nA.KeysAdd("addr-A")
	nA.BankSendFromNode(s.addrA, "10000000ubbn")
	icaOwnerAccount := s.addrA
	icaOwnerPortID, _ := icatypes.NewControllerPortID(icaOwnerAccount)

	nA.WaitForNextBlock()

	balanceBeforeSendAddrA, err := nA.QueryBalances(s.addrA)
	s.Require().NoError(err)
	// Confirm val on A has enough funds
	s.Assert().GreaterOrEqual(balanceBeforeSendAddrA.AmountOf(nativeDenom).Int64(), amount)

	// register ICA account
	txHash := nA.RegisterICAAccount(icaOwnerAccount, icaConnectionID)
	nA.WaitForNextBlock()

	_, txResp := nA.QueryTx(txHash)
	registerTxFees := txResp.AuthInfo.Fee.Amount

	// setup ICA connection
	err = s.configurer.CompleteIBCChannelHandshake(
		bbnChainA.ChainMeta.Id,
		bbnChainB.ChainMeta.Id,
		icaConnectionID,
		icaConnectionID,
		icaOwnerPortID,
		icatypes.HostPortID,
		icaChannel,
		icaChannel,
	)
	s.Require().NoError(err)

	var icaAccount string
	s.Require().Eventually(
		func() bool {
			icaAccount = nA.QueryICAAccountAddress(icaOwnerAccount, icaConnectionID)
			return icaAccount != ""
		},
		time.Minute,
		5*time.Second,
	)

	// Send transfer from val in chain-A (Node 3) to ICA account in chain-B
	txHash = nA.SendIBCTransfer(s.addrA, icaAccount, "transfer", transferCoin)
	nA.WaitForNextBlock()

	_, txResp = nA.QueryTx(txHash)
	ibcTxFees := txResp.AuthInfo.Fee.Amount
	totalFeesPaid := registerTxFees.Add(ibcTxFees...)
	s.Require().Eventually(func() bool {
		// Check that the transfer is successful.
		// Amounts have been discounted from val in chain-A and added (as a wrapped denom) to icaAccount in chain-B
		balanceAfterSendAddrA, err := nA.QueryBalances(s.addrA)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}

		expectedAmt := balanceBeforeSendAddrA.Sub(transferCoin).Sub(totalFeesPaid...).String()
		actualAmt := balanceAfterSendAddrA.String()

		if !strings.EqualFold(expectedAmt, actualAmt) {
			s.T().Logf(
				"BalanceBeforeSendAddrA: %s; BalanceAfterSendAddrA: %s, txFees: %s, coinTransfer: %s",
				balanceBeforeSendAddrA.String(), balanceAfterSendAddrA.String(), registerTxFees.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer was not successful")

	s.Require().Eventually(func() bool {
		balanceAfterSendICAAcc, err := nB.QueryBalances(icaAccount)
		if err != nil {
			s.T().Logf("failed to query balances: %s", err.Error())
			return false
		}

		denomB := getFirstIBCDenom(balanceAfterSendICAAcc)
		if !balanceAfterSendICAAcc.AmountOf(denomB).Equal(transferCoin.Amount) {
			s.T().Logf(
				"BalanceAfterSendICAAcc: %s, txFees: %s, coinTransfer: %s",
				balanceAfterSendICAAcc.String(), registerTxFees.String(), transferCoin.String(),
			)
			return false
		}

		return true
	}, 1*time.Minute, 1*time.Second, "Transfer was not successful")
}
