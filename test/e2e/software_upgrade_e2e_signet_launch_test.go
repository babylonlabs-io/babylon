package e2e

import (
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/config"
)

type SoftwareUpgradeSignetLaunchTestSuite struct {
	suite.Suite

	configurer *configurer.UpgradeConfigurer
}

func (s *SoftwareUpgradeSignetLaunchTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	btcHeaderGenesis, err := app.SignetBtcHeaderGenesis(app.NewTmpBabylonApp().AppCodec())
	s.NoError(err)

	cfg, err := configurer.NewSoftwareUpgradeConfigurer(s.T(), true, config.UpgradeSignetLaunchFilePath, []*btclighttypes.BTCHeaderInfo{btcHeaderGenesis})
	s.NoError(err)
	s.configurer = cfg

	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup() // upgrade happens at the setup of configurer.
	s.Require().NoError(err)
}

func (s *SoftwareUpgradeSignetLaunchTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestUpgradeSignetLaunch Checks if the BTC Headers were inserted.
func (s *SoftwareUpgradeSignetLaunchTestSuite) TestUpgradeSignetLaunch() {
	// chain is already upgraded, only checks for differences in state are expected
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(30) // five blocks more than upgrade

	n, err := chainA.GetDefaultNode()
	s.NoError(err)

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)

	// makes sure that the upgrade was actually executed
	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v1.Upgrade.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	btcHeadersInserted, err := v1.LoadBTCHeadersFromData()
	s.NoError(err)

	lenHeadersInserted := len(btcHeadersInserted)
	oldHeadersStoredLen := 1 // only block zero is set by default in genesis for e2e test

	storedBtcHeadersResp := n.QueryBtcLightClientMainchain()
	storedHeadersLen := len(storedBtcHeadersResp)
	s.Equal(storedHeadersLen, oldHeadersStoredLen+lenHeadersInserted)

	// ensure the headers were inserted at the end
	for i := 0; i < lenHeadersInserted; i++ {
		headerInserted := btcHeadersInserted[i]
		reversedStoredIndex := storedHeadersLen - (oldHeadersStoredLen + i + 1)
		headerStoredResp := storedBtcHeadersResp[reversedStoredIndex] // reverse reading

		s.EqualValues(headerInserted.Header.MarshalHex(), headerStoredResp.HeaderHex)
	}
}
