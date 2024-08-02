package e2e

import (
	"github.com/stretchr/testify/suite"

	v1 "github.com/babylonlabs-io/babylon/app/upgrades/vanilla"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/config"
)

type SoftwareUpgradeVanillaTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *SoftwareUpgradeVanillaTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	s.configurer, err = configurer.NewSoftwareUpgradeConfigurer(s.T(), false, config.UpgradeVanillaFilePath, nil)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup() // upgrade happens at the setup of configurer.
	s.NoError(err)
}

func (s *SoftwareUpgradeVanillaTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	s.Require().NoError(err)
}

// TestUpgradeVanilla only checks that new fp was added.
func (s *SoftwareUpgradeVanillaTestSuite) TestUpgradeVanilla() {
	// chain is already upgraded, only checks for differences in state are expected
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(30) // five blocks more than upgrade

	n, err := chainA.GetDefaultNode()
	s.NoError(err)

	expectedUpgradeHeight := int64(25)

	// makes sure that the upgrade was actually executed
	resp := n.QueryAppliedPlan(v1.Upgrade.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height 25")

	fps := n.QueryFinalityProviders()
	s.Len(fps, 1, "it should have one finality provider, since the vanilla upgrade just added a new one")
}
