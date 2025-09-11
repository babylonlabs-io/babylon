package testnet_test

import (
	"testing"
	"time"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/math"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	v3rc4testnet "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc4/testnet"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/epoching"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
)

const (
	DummyUpgradeHeight = 11 // Must be at epoch boundary for epochInterval=10
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
	// Add the upgrade plan
	app.Upgrades = []upgrades.Upgrade{v3rc4testnet.Upgrade}

	// Set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())
}

func (s *UpgradeTestSuite) TestUpgradeValidation() {
	testCases := []struct {
		name                  string
		upgradeHeight         int64
		expectSuccess         bool
		expectedErrorContains string
	}{
		{
			name:          "successful upgrade at epoch boundary",
			upgradeHeight: 11, // First block of epoch 2 when epochInterval=10
			expectSuccess: true,
		},
		{
			name:                  "failed upgrade at non-epoch boundary",
			upgradeHeight:         12, // Not first block of epoch
			expectSuccess:         false,
			expectedErrorContains: "epoch boundary validation failed",
		},
		{
			name:          "successful upgrade at different epoch boundary",
			upgradeHeight: 21, // First block of epoch 3 when epochInterval=10
			expectSuccess: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // Reset for each test

			if tc.expectSuccess {
				s.executeSuccessfulUpgrade(tc.upgradeHeight)
			} else {
				s.executeFailedUpgrade(tc.upgradeHeight, tc.expectedErrorContains)
			}
		})
	}
}

func (s *UpgradeTestSuite) TestUpgradeAtHeight1() {
	s.SetupTest()

	// Height 1 should always be valid (special case in validation)
	s.executeSuccessfulUpgrade(1)
}

func (s *UpgradeTestSuite) TestUpgradeFailureScenarios() {
	testCases := []struct {
		name                  string
		upgradeHeight         int64
		setupFailureCondition func()
		expectedErrorContains string
	}{
		{
			name:          "upgrade fails with locked delegate pool funds",
			upgradeHeight: DummyUpgradeHeight,
			setupFailureCondition: func() {
				// Simulate locked funds in delegate pool
				coins := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(500000)))
				s.Require().NoError(s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, coins))
				s.Require().NoError(s.app.BankKeeper.SendCoinsFromModuleToModule(s.ctx, minttypes.ModuleName, epochingtypes.DelegatePoolModuleName, coins))
			},
			expectedErrorContains: "delegate pool validation failed",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // Reset for each test

			if tc.setupFailureCondition != nil {
				tc.setupFailureCondition()
			}

			s.executeFailedUpgrade(tc.upgradeHeight, tc.expectedErrorContains)
		})
	}
}

// ========== HELPER FUNCTIONS ==========

// advanceToEpochBoundary advances the context to the specified height by progressing through blocks
func (s *UpgradeTestSuite) advanceToEpochBoundary(targetHeight int64) {
	for currentHeight := s.ctx.BlockHeight(); currentHeight < targetHeight; currentHeight++ {
		s.ctx = s.ctx.WithBlockHeight(currentHeight + 1).WithHeaderInfo(header.Info{
			Height: currentHeight + 1,
			Time:   s.ctx.BlockTime().Add(time.Second),
		})

		// Process epoch transitions
		err := epoching.BeginBlocker(s.ctx, s.app.EpochingKeeper)
		s.Require().NoError(err)
	}
}

func (s *UpgradeTestSuite) executeUpgrade(upgradeHeight int64) error {
	currentHeight := s.ctx.BlockHeight()

	// Advance to target height if needed
	if upgradeHeight > currentHeight+1 {
		s.advanceToEpochBoundary(upgradeHeight - 1)
	}

	// Schedule upgrade
	s.ctx = s.ctx.WithBlockHeight(upgradeHeight - 1)
	plan := upgradetypes.Plan{Name: v3rc4testnet.UpgradeName, Height: upgradeHeight}
	err := s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan)
	s.Require().NoError(err)

	// Verify upgrade plan exists
	actualPlan, err := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(plan, actualPlan)

	// Execute upgrade
	s.ctx = s.ctx.WithHeaderInfo(header.Info{Height: upgradeHeight, Time: s.ctx.BlockTime().Add(time.Second)}).WithBlockHeight(upgradeHeight)

	var upgradeErr error
	s.NotPanics(func() {
		_, upgradeErr = s.preModule.PreBlock(s.ctx)
	})

	return upgradeErr
}

func (s *UpgradeTestSuite) executeSuccessfulUpgrade(upgradeHeight int64) {
	err := s.executeUpgrade(upgradeHeight)
	s.Require().NoError(err)
}

func (s *UpgradeTestSuite) executeFailedUpgrade(upgradeHeight int64, expectedErrorContains string) {
	err := s.executeUpgrade(upgradeHeight)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), expectedErrorContains)
}
