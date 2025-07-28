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
	ctx              sdk.Context
	app              *app.BabylonApp
	preModule        appmodule.HasPreBlocker
	initialBtcHeight uint64
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) setupTestWithNetwork(isMainnet bool) {
	s.initialBtcHeight = 100

	app.Upgrades = []upgrades.Upgrade{CreateUpgrade(isMainnet)}

	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	// Set initial BTC staking parameters with a known BtcActivationHeight
	params := s.app.BTCStakingKeeper.GetParams(s.ctx)
	params.BtcActivationHeight = uint32(s.initialBtcHeight)
	err := s.app.BTCStakingKeeper.SetParams(s.ctx, params)
	s.Require().NoError(err)
}

func (s *UpgradeTestSuite) executeUpgrade() {
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

func (s *UpgradeTestSuite) TestUpgradeNetworks() {
	testCases := []struct {
		name                    string
		isMainnet               bool
		expectedMaxFPs          uint32
		expectedHeightIncrement uint32
	}{
		{
			name:                    "mainnet upgrade",
			isMainnet:               true,
			expectedMaxFPs:          5,
			expectedHeightIncrement: 288,
		},
		{
			name:                    "testnet upgrade",
			isMainnet:               false,
			expectedMaxFPs:          10,
			expectedHeightIncrement: 144,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.setupTestWithNetwork(tc.isMainnet)

			s.executeUpgrade()

			s.verifyPostUpgrade(tc.expectedMaxFPs, tc.expectedHeightIncrement)
		})
	}
}

func (s *UpgradeTestSuite) verifyPostUpgrade(expectedMaxFPs, expectedHeightIncrement uint32) {
	_, found := s.app.ModuleManager.Modules[deletedCapabilityStoreKey]
	s.Require().False(found, "x/capability module should be deleted")

	_, found = s.app.ModuleManager.Modules["btcstkconsumer"]
	s.Require().True(found, "x/btcstkconsumer module should be found")

	_, found = s.app.ModuleManager.Modules["zoneconcierge"]
	s.Require().True(found, "x/zoneconcierge module should be found")

	params := s.app.BTCStakingKeeper.GetParams(s.ctx)
	s.Require().Equal(expectedMaxFPs, params.MaxFinalityProviders, "MaxFinalityProviders should match expected value")

	expectedBtcHeight := uint32(s.initialBtcHeight) + expectedHeightIncrement
	s.Require().Equal(expectedBtcHeight, params.BtcActivationHeight, "BtcActivationHeight should be incremented correctly")
}

// Legacy test methods for backwards compatibility
func (s *UpgradeTestSuite) SetupTest() {
	s.setupTestWithNetwork(false) // Default to testnet
}

func (s *UpgradeTestSuite) Upgrade() {
	s.executeUpgrade()
}

func (s *UpgradeTestSuite) TestUpgrade() {
	s.SetupTest()
	s.PreUpgrade()
	s.Upgrade()
	s.PostUpgrade()
}

func (s *UpgradeTestSuite) PreUpgrade() {}

func (s *UpgradeTestSuite) PostUpgrade() {
	s.verifyPostUpgrade(10, 144) // Testnet values: 10 MaxFPs, 144 height increment
}
