package e2e

import (
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

func (s *IBCTransferTestSuite) Test1IBCTransfer() {
	denom := "ubbn"
	amount := int64(1_000_000)
	delta := float64(10000) // Tolerance to account for gas fees

	transferAmount := sdk.NewInt64Coin(denom, amount)

	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	babylonNodeA, err := bbnChainA.GetNodeAtIndex(2)
	s.NoError(err)
	babylonNodeB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	val := initialization.ValidatorWalletName

	// Check balance of val in chain-A (Node 3)
	addrA := babylonNodeA.GetWallet(val)
	balanceA, err := babylonNodeA.QueryBalances(addrA)
	s.Require().NoError(err)
	// Confirm val on A has enough funds
	s.Assert().GreaterOrEqual(balanceA.AmountOf(denom).Int64(), amount)

	addrB := babylonNodeB.GetWallet(val)
	balanceB, err := babylonNodeB.QueryBalances(addrB)
	s.Require().NoError(err)
	// Only one denom in B
	s.Require().Len(balanceB, 1)

	// Send transfer from val in chain-A (Node 3) to val in chain-B
	babylonNodeA.SendIBCTransfer(val, addrB, "", transferAmount)

	time.Sleep(10 * time.Second)

	// Check the transfer is successful.
	// Amounts have been discounted from val in chain-A and added (as a wrapped denom) to val in chain-B
	balanceA2, err := babylonNodeA.QueryBalances(addrA)
	s.Require().NoError(err)
	s.Assert().InDelta(balanceA.Sub(transferAmount).AmountOf(denom).Int64(), balanceA2.AmountOf(denom).Int64(), delta)

	balanceB2, err := babylonNodeB.QueryBalances(addrB)
	s.Require().NoError(err)
	// Check that there are now two denoms in B
	s.Require().Len(balanceB2, 2)
	// Look for the ugly IBC one
	denomsB := balanceB2.Denoms()
	var denomB string
	for _, d := range denomsB {
		if strings.HasPrefix(d, "ibc/") {
			denomB = d
			break
		}
	}
	// Check the balance of the IBC denom
	s.Assert().InDelta(balanceB2.AmountOf(denomB).Int64(), transferAmount.Amount.Int64(), delta)
}
