package e2e

import (
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

func (s *IBCTransferTestSuite) Test1IBCTransfer() {
	val := initialization.ValidatorWalletName
	denom := "ubbn"
	amount := int64(1_000_000)
	tol := 0.01 // 1% tolerance to account for gas fees
	delta := float64(amount) * tol
	transferAmount := sdk.NewInt64Coin(denom, amount)

	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	babylonNodeA, err := bbnChainA.GetNodeAtIndex(2)
	s.NoError(err)
	babylonNodeB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	// Check balance of val in chain-A (Node 3)
	addrA := babylonNodeA.GetWallet(val)
	balanceA, err := babylonNodeA.QueryBalances(addrA)
	s.Require().NoError(err)
	// Confirm val on A has enough funds
	s.Assert().GreaterOrEqual(balanceA.AmountOf(denom).Int64(), amount)

	addrB := babylonNodeB.GetWallet(val)
	balanceB, err := babylonNodeB.QueryBalances(addrB)
	s.Require().NoError(err)

	// Send transfer from val in chain-A (Node 3) to val in chain-B
	babylonNodeA.SendIBCTransfer(val, addrB, "", transferAmount)

	time.Sleep(1 * time.Minute)

	// Check the transfer is successful. Amounts have been discounted from val in chain-A and added to val in chain-B
	balanceA2, err := babylonNodeA.QueryBalances(addrA)
	s.Require().NoError(err)
	s.Assert().InDelta(balanceA.Sub(transferAmount).AmountOf(denom).Int64(), balanceA2.AmountOf(denom).Int64(), delta)

	balanceB2, err := babylonNodeB.QueryBalances(addrB)
	s.Require().NoError(err)
	s.Assert().InDelta(balanceB.Add(transferAmount).AmountOf(denom).Int64(), balanceB2.AmountOf(denom).Int64(), delta)
}
