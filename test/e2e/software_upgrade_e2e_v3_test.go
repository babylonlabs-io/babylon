package e2e

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
	"github.com/babylonlabs-io/babylon/v3/testutil/sample"
	btclighttypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/config"
)

const (
	btcstkconsumerModulePath = "btcstkconsumer"
	zoneconciergeModulePath  = "zoneconcierge"
)

type SoftwareUpgradeV3TestSuite struct {
	suite.Suite

	configurer            *configurer.UpgradeConfigurer
	balancesBeforeUpgrade map[string]sdk.Coin
}

func (s *SoftwareUpgradeV3TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite for v2.2.0 to v3 upgrade...")
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
		config.UpgradeV3FilePath,
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

func (s *SoftwareUpgradeV3TestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestUpgradeV3 checks if the upgrade from v2.2.0 to v3 was successful
func (s *SoftwareUpgradeV3TestSuite) TestUpgradeV3() {
	chainA := s.configurer.GetChainConfig(0)

	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1) // waits for chain to produce blocks

	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v3.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	// check that the module exists by querying parameters with the QueryParams helper
	var btcstkconsumerParams map[string]interface{}
	n.QueryParams(btcstkconsumerModulePath, &btcstkconsumerParams)
	s.T().Logf("btcstkconsumer params: %v", btcstkconsumerParams)

	finalisedChainsInfoResp := n.QueryZoneConciergeFinalizedChainsInfo([]string{}, false)
	s.NoError(err, "zoneconcierge FinalizedChainsInfo query should succeed")
	s.T().Logf("zoneconcierge FinalizedChainsInfo: %v", finalisedChainsInfoResp)

	n.WaitForNextBlock()

	// TODO: Add more functionality checks here as they are added
}
