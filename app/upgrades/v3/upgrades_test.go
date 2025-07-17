package v3

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

const (
	DummyUpgradeHeight = 5
)

type UpgradeTestSuite struct {
	suite.Suite
	ctx       sdk.Context
	app       *app.BabylonApp
	preModule appmodule.HasPreBlocker
}

func TestUpdateTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) SetupTest() {
	app.Upgrades = []upgrades.Upgrade{CreateUpgrade()}

	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())
}

func (s *UpgradeTestSuite) Upgrade() {
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: UpgradeName, Height: DummyUpgradeHeight}
	err := s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan)
	s.Require().NoError(err)

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

func (s *UpgradeTestSuite) TestUpgrade() {
	s.SetupTest()
	s.PreUpgrade()
	s.Upgrade()
	s.PostUpgrade()
}

func (s *UpgradeTestSuite) PreUpgrade() {}

func (s *UpgradeTestSuite) PostUpgrade() {
	_, found := s.app.ModuleManager.Modules[deletedCapabilityStoreKey]
	s.Require().False(found, "x/capability module shouldn't be found")

	_, found = s.app.ModuleManager.Modules["btcstkconsumer"]
	s.Require().True(found, "x/btcstkconsumer module shouldn't be found")

	_, found = s.app.ModuleManager.Modules["zoneconcierge"]
	s.Require().True(found, "x/zoneconcierge module shouldn't be found")

	params := s.app.BTCStakingKeeper.GetParams(s.ctx)
	s.Require().Equal(uint32(1), params.MaxFinalityProviders)
}
