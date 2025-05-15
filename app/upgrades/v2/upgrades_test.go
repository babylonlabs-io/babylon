package v2_test

import (
	_ "embed"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	"github.com/babylonlabs-io/babylon/v2/testutil/sample"
	bbn "github.com/babylonlabs-io/babylon/v2/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v2/app"
	v2 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v2"
	"github.com/babylonlabs-io/babylon/v2/test/e2e/configurer"
	btclighttypes "github.com/babylonlabs-io/babylon/v2/x/btclightclient/types"
)

const (
	DummyUpgradeHeight = 5
)

var ()

type UpgradeTestSuite struct {
	suite.Suite

	ctx        sdk.Context
	app        *app.BabylonApp
	preModule  appmodule.HasPreBlocker
	configurer configurer.Configurer
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) SetupSuite() {
	s.T().Log("setting up upgrade test suite...")
	var (
		err error
	)

	s.configurer, err = configurer.NewIBCTransferConfigurer(s.T(), true)

	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)

	s.SetupWithMultipleIBCChannels()
}

func (s *UpgradeTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *UpgradeTestSuite) SetupWithMultipleIBCChannels() {

	// chainA := s.configurer.GetChainConfig(0)
	// chainB := s.configurer.GetChainConfig(1)

	// for i := 1; i <= 5; i++ {
	// 	portA := fmt.Sprintf("channel-%d", i)
	// 	portB := fmt.Sprintf("channel-%d", i)

	// 	// Log the expected channel name
	// 	expectedChannelName := fmt.Sprintf("channel-%d", i)
	// 	s.T().Logf("Creating IBC channel: %s", expectedChannelName)

	// 	cmd := []string{
	// 		"hermes", "create", "channel",
	// 		"--a-chain", chainA.ChainMeta.Id,
	// 		"--b-chain", chainB.ChainMeta.Id,
	// 		"--a-port", portA,
	// 		"--b-port", portB,
	// 		"--new-client-connection",
	// 		"--yes",
	// 	}
	// 	s.T().Log(cmd)

	// 	stdout, stderr, err := s.configurer.ContainerManager().ExecHermesCmd(s.T(), cmd, "SUCCESS")
	// 		s.Require().NoError(err, "Failed to create IBC channel %s", expectedChannelName)
	// 	} else {
	// 		s.T().Logf("Successfully created IBC channel: %s", expectedChannelName)
	// 		s.T().Logf("Hermes stdout: %s", stdout.String())
	// 	}
	// }
}

func (s *UpgradeTestSuite) TestUpgrade() {
	testCases := []struct {
		msg           string
		baseBtcHeader *btclighttypes.BTCHeaderInfo
		preUpgrade    func()
		upgrade       func()
		postUpgrade   func()
	}{
		{
			"Test launch software upgrade v1 mainnet",
			sample.MainnetBtcHeader854784(s.T()),
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			s.SetupTest() // reset

			tc.preUpgrade()
			tc.upgrade()
			tc.postUpgrade()
		})
	}
}

func (s *UpgradeTestSuite) SetupTest() {
	// add the upgrade plan
	app.Upgrades = []upgrades.Upgrade{v2.Upgrade}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())
}

func (s *UpgradeTestSuite) PreUpgrade() {}

func (s *UpgradeTestSuite) Upgrade() {
	// inject upgrade plan
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: v2.UpgradeName, Height: DummyUpgradeHeight}
	err := s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan)
	s.NoError(err)

	// ensure upgrade plan exists
	actualPlan, err := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
	s.NoError(err)
	s.Equal(plan, actualPlan)

	// execute upgrade
	s.ctx = s.ctx.WithHeaderInfo(header.Info{Height: DummyUpgradeHeight, Time: s.ctx.BlockTime().Add(time.Second)}).WithBlockHeight(DummyUpgradeHeight)
	s.NotPanics(func() {
		_, err := s.preModule.PreBlock(s.ctx)
		s.Require().NoError(err)
	})
}

func (s *UpgradeTestSuite) PostUpgrade() {}
