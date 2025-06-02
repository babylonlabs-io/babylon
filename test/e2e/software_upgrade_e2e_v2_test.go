package e2e

import (
	"github.com/babylonlabs-io/babylon/v4/x/mint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2"
	"github.com/babylonlabs-io/babylon/v4/testutil/sample"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/config"
)

const (
	TokenFactoryModulePath = "tokenfactory"
)

type SoftwareUpgradeV2TestSuite struct {
	suite.Suite

	configurer            *configurer.UpgradeConfigurer
	balancesBeforeUpgrade map[string]sdk.Coin
}

func (s *SoftwareUpgradeV2TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite for v1.1 to v2 upgrade...")
	var err error
	s.balancesBeforeUpgrade = make(map[string]sdk.Coin)

	btcHeaderGenesis := sample.SignetBtcHeader195552(s.T())

	// func runs right before the upgrade proposal is sent
	preUpgradeFunc := func(chains []*chain.Config) {
		node := chains[0].NodeConfigs[1]

		// Record some balances before the upgrade to verify after
		addresses := []string{
			node.PublicAddress,
			chains[0].NodeConfigs[0].PublicAddress,
		}

		for _, addr := range addresses {
			balance, err := node.QueryBalance(addr, appparams.DefaultBondDenom)
			s.NoError(err)
			s.balancesBeforeUpgrade[addr] = *balance
		}
	}

	cfg, err := configurer.NewSoftwareUpgradeConfigurer(
		s.T(),
		true,
		config.UpgradeV2FilePath,
		[]*btclighttypes.BTCHeaderInfo{btcHeaderGenesis},
		preUpgradeFunc,
	)
	s.NoError(err)
	s.configurer = cfg

	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup() // upgrade happens at the setup of configurer.
	s.Require().NoError(err)
}

func (s *SoftwareUpgradeV2TestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestUpgradeV2 checks if the upgrade from v1.1 to v2 was successful
func (s *SoftwareUpgradeV2TestSuite) TestUpgradeV2() {
	// Chain is already upgraded, check for new modules and state changes
	chainA := s.configurer.GetChainConfig(0)

	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1) // waits for chain to produce blocks

	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v2.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	// Check that the module exists by querying parameters with the QueryParams helper
	var tokenfactoryParams map[string]interface{}
	n.QueryParams(TokenFactoryModulePath, &tokenfactoryParams)
	s.T().Logf("Tokenfactory params: %v", tokenfactoryParams)

	params, ok := tokenfactoryParams["params"].(map[string]interface{})
	s.Require().True(ok, "params field should exist and be a map")

	denomCreationFee, ok := params["denom_creation_fee"].([]interface{})
	s.Require().True(ok, "denom_creation_fee should be a list")
	s.Require().Len(denomCreationFee, 1, "denom_creation_fee should have one entry")

	feeEntry, ok := denomCreationFee[0].(map[string]interface{})
	s.Require().True(ok, "fee entry should be a map")
	s.Equal(types.DefaultBondDenom, feeEntry["denom"])
	s.Equal("10000000", feeEntry["amount"])

	s.Equal("2000000", params["denom_creation_gas_consume"])

	n.WaitForNextBlock()

	// TODO: Add more functionality checks here as they are added
}
