package v5_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/v5"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
)

const (
	DummyUpgradeHeight = 8
)

type UpgradeTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	app       *app.BabylonApp
	preModule appmodule.HasPreBlocker
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) SetupTest() {
	// add the upgrade plan
	app.Upgrades = []upgrades.Upgrade{v5.Upgrade}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	// simulate pre-upgrade state by setting btcstaking version to 1
	// and resetting multisig params to 1
	vm, err := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
	s.NoError(err)
	vm["btcstaking"] = 1
	s.app.UpgradeKeeper.SetModuleVersionMap(s.ctx, vm)

	// reset multisig params to simulate pre-upgrade state
	params := s.app.BTCStakingKeeper.GetParams(s.ctx)
	storedParams := s.app.BTCStakingKeeper.GetParamsWithVersion(s.ctx)
	params.MaxStakerQuorum = 0
	params.MaxStakerNum = 0
	err = s.app.BTCStakingKeeper.OverwriteParamsAtVersion(s.ctx, storedParams.Version, params)
	s.NoError(err)
}

func (s *UpgradeTestSuite) TestUpgrade() {
	testCases := []struct {
		msg         string
		preUpgrade  func()
		upgrade     func()
		postUpgrade func()
	}{
		{
			"Test v5 upgrade with multisig params migration",
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()

				vm, err := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
				s.NoError(err)
				s.Equal(uint64(2), vm["btcstaking"], "btcstaking should be version 2 after upgrade")

				params := s.app.BTCStakingKeeper.GetParams(s.ctx)
				s.Equal(uint32(2), params.MaxStakerQuorum, "MaxStakerQuorum should be 2")
				s.Equal(uint32(3), params.MaxStakerNum, "MaxStakerNum should be 3")

				err = params.Validate()
				s.NoError(err, "migrated params should be valid")

				s.Equal(uint32(3), params.CovenantQuorum, "CovenantQuorum should remain unchanged")
				s.Equal(int(params.MinStakingValueSat), 10000, "MinStakingValueSat should remain set")
			},
		},
		{
			"Test v5 upgrade preserves existing params",
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()

				// Get params after upgrade
				params := s.app.BTCStakingKeeper.GetParams(s.ctx)

				// Verify all existing params are still present and valid
				s.Equal(len(params.CovenantPks), 5, "CovenantPks should be preserved")
				s.Equal(int(params.CovenantQuorum), 3, "CovenantQuorum should be preserved")
				s.Equal(int(params.MinStakingValueSat), 10000, "MinStakingValueSat should be preserved")
				s.Equal(params.MaxStakingValueSat, int64(10*10e8), "MaxStakingValueSat should be preserved")
				s.Equal(int(params.MinStakingTimeBlocks), 400, "MinStakingTimeBlocks should be preserved")
				s.Equal(int(params.MaxStakingTimeBlocks), math.MaxUint16, "MaxStakingTimeBlocks should be preserved")
				s.NotEmpty(params.SlashingPkScript, "SlashingPkScript should be preserved")
				s.Equal(int(params.MinSlashingTxFeeSat), 1000, "MinSlashingTxFeeSat should be preserved")
				s.False(params.SlashingRate.IsNil(), "SlashingRate should be preserved")
				s.Equal(int(params.UnbondingTimeBlocks), 200, "UnbondingTimeBlocks should be preserved")
				s.Equal(int(params.UnbondingFeeSat), 1000, "UnbondingFeeSat should be preserved")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.msg, func() {
			s.SetupTest() // reset for each test case

			tc.preUpgrade()
			tc.upgrade()
			tc.postUpgrade()
		})
	}
}

func (s *UpgradeTestSuite) PreUpgrade() {
	vm, err := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
	s.NoError(err)
	btcstakingVersion, exists := vm["btcstaking"]
	s.True(exists, "btcstaking module should exist")
	s.Equal(uint64(1), btcstakingVersion, "btcstaking should be version 1 before upgrade")
}

func (s *UpgradeTestSuite) Upgrade() {
	// inject upgrade plan
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: v5.UpgradeName, Height: DummyUpgradeHeight}
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
