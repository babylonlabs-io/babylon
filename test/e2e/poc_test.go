package e2e

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v2/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v2/test/e2e/initialization"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
)

type PocTestSuite struct {
	suite.Suite

	configurer  configurer.Configurer
	valAccAddrA string
	valAccAddrB string
}

func TestPocTest(t *testing.T) {
	suite.Run(t, new(PocTestSuite))
}

func (s *PocTestSuite) SetupSuite() {
	s.T().Log("setting up PoC test suite...")
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

func (s *PocTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *PocTestSuite) TestPoc() {
	chainA := s.configurer.GetChainConfig(0)
	chainB := s.configurer.GetChainConfig(1)

	// Node A, which custom denom (utest) with supply of MAX_INT_256 already minted
	nodeA, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	s.valAccAddrA = nodeA.GetWallet("val")

	nodeB, err := chainB.GetNodeAtIndex(2)
	s.NoError(err)
	s.valAccAddrB = nodeB.GetWallet("val")

	denomA := initialization.TestDenom
	maxSupply, ok := sdkmath.NewIntFromString(initialization.MaxSupply)
	s.Require().True(ok)
	transferCoin := sdk.NewCoin(denomA, maxSupply)

	// Transfer test denom from chain A to chain B
	nodeA.SendIBCTransfer(s.valAccAddrA, s.valAccAddrB, "transfer", transferCoin)
	nodeA.WaitForNextBlock()

	// Wait until denom is received on chain B
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", "channel-0", denomA))
	ibcDenomA := denomTrace.IBCDenom()
	s.Require().Eventually(func() bool {
		balances, err := nodeB.QueryBalances(s.valAccAddrB)
		if err != nil {
			return false
		}
		ibcDenomAAmount := balances.AmountOf(ibcDenomA)
		return ibcDenomAAmount.String() == maxSupply.String()
	}, 3*time.Minute, 1*time.Second, "Transfer was not successful")

	// Use test denom to fund validator rewards pool
	valAddrB := sdk.ValAddress(sdk.MustAccAddressFromBech32(s.valAccAddrB)).String()
	rewardsAmount := sdk.NewCoin(ibcDenomA, maxSupply).String()
	nodeB.FundValidatorRewardsPool(s.valAccAddrB, valAddrB, rewardsAmount)
	nodeB.WaitForNextBlock()

	// Withdraw validator rewards for validator
	nodeB.WithdrawValidatorRewards(s.valAccAddrB, valAddrB)
	nodeB.WaitForNextBlock()

	// Increase stake so node becomes bonded, initial stake was 1 token
	stakeAmount := initialization.StakeAmountCoinB.String()
	nodeB.Delegate(s.valAccAddrB, valAddrB, stakeAmount)
	nodeB.WaitForNextBlocks(10)

	// Fund more validator rewards
	nodeB.FundValidatorRewardsPool(s.valAccAddrB, valAddrB, rewardsAmount)
	nodeB.WaitForNextBlocks(10)

	// Increase stake for node 1 so it has majority of stake, so chain keeps running while we set up double sign scenario
	// Node 0 and 2 are being used for double sign testing
	nodeB1, err := chainB.GetNodeAtIndex(1)
	s.NoError(err)
	valAccAddrB1 := nodeB1.GetWallet("val")
	valAddrB1 := sdk.ValAddress(sdk.MustAccAddressFromBech32(valAccAddrB1)).String()
	stakeAmount = sdk.NewCoin(initialization.BabylonDenom, sdkmath.NewInt(2500000000000)).String()
	nodeB1.Delegate(valAccAddrB1, valAddrB1, stakeAmount)
	nodeB1.WaitForNextBlocks(10)

	// Replace keys on node0 to trigger double sign
	privKeyFile := nodeB.ReadPrivValKeyFile()
	nodeB0, err := chainB.GetNodeAtIndex(0)
	s.NoError(err)
	nodeB0.WritePrivValKeyFile(privKeyFile)

	err = nodeB0.Stop()
	s.NoError(err)

	err = nodeB0.Run()
	s.NoError(err)
	time.Sleep(5 * time.Minute)

	// Search docker logs on any chain B node for "Int overflow" or "ERR CONSENSUS FAILURE!!!"
}
