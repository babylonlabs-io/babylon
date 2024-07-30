package e2e

import (
	"github.com/stretchr/testify/suite"

	v1 "github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/config"
)

type SoftwareUpgradeSignetLaunchTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *SoftwareUpgradeSignetLaunchTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	s.configurer, err = configurer.NewSoftwareUpgradeConfigurer(s.T(), false, config.UpgradeSignetLaunchFilePath)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup() // upgrade happens at the setup of configurer.
	s.NoError(err)
}

func (s *SoftwareUpgradeSignetLaunchTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	s.Require().NoError(err)
}

// TestUpgradeSignetLaunch Checks if the BTC Headers were inserted.
func (s *SoftwareUpgradeSignetLaunchTestSuite) TestUpgradeSignetLaunch() {
	// chain is already upgraded, only checks for differences in state are expected
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(30) // five blocks more than upgrade

	n, err := chainA.GetDefaultNode()
	s.NoError(err)

	expectedUpgradeHeight := int64(25)

	// makes sure that the upgrade was actually executed
	resp := n.QueryAppliedPlan(v1.Upgrade.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height 25")

	btcHeadersInserted, err := v1.LoadBTCHeadersFromData()
	s.NoError(err)

	lenHeadersInserted := len(btcHeadersInserted)
	oldHeadersStoredLen := 1 // only block zero is set by default in genesis for e2e test

	storedBtcHeadersResp := n.QueryBtcLightClientMainchain()
	newHeadersLen := len(storedBtcHeadersResp)
	s.Equal(newHeadersLen, oldHeadersStoredLen+lenHeadersInserted)

	// ensure the headers were inserted at the end
	for i := 0; i < newHeadersLen-lenHeadersInserted; i++ {
		headerInserted := btcHeadersInserted[i]
		headerStoredResp := storedBtcHeadersResp[oldHeadersStoredLen+i]

		s.EqualValues(headerInserted.ToResponse(), headerStoredResp)
	}
}
