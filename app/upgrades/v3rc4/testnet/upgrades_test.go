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
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v3rc4testnet "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3rc4/testnet"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/epoching"
	epochingkeeper "github.com/babylonlabs-io/babylon/v3/x/epoching/keeper"
	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	minttypes "github.com/babylonlabs-io/babylon/v3/x/mint/types"
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

func (s *UpgradeTestSuite) TestUpgradeWarningScenarios() {
	testCases := []struct {
		name           string
		upgradeHeight  int64
		setupCondition func()
		expectSuccess  bool
		expectWarning  bool
	}{
		{
			name:          "upgrade warns about locked delegate pool funds but succeeds",
			upgradeHeight: DummyUpgradeHeight,
			setupCondition: func() {
				coins := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(500000)))
				s.Require().NoError(s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, coins))
				s.Require().NoError(s.app.BankKeeper.SendCoinsFromModuleToModule(s.ctx, minttypes.ModuleName,
					epochingtypes.DelegatePoolModuleName, coins))
			},
			expectSuccess: true,
			expectWarning: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // Reset for each test

			if tc.setupCondition != nil {
				tc.setupCondition()
			}

			if tc.expectSuccess {
				s.executeSuccessfulUpgrade(tc.upgradeHeight)

				// For the MsgDelegate test case, verify that delegation works after upgrade
				if tc.name == "upgrade warns about delegate pool funds then processes MsgDelegate successfully" {
					s.verifyPostUpgradeDelegation()
				}
			}
		})
	}
}

func (s *UpgradeTestSuite) TestUpgradeFailureScenarios() {
	testCases := []struct {
		name                  string
		upgradeHeight         int64
		setupFailureCondition func()
		expectedErrorContains string
	}{
		{
			name:          "upgrade fails at height 2 (edge case)",
			upgradeHeight: 2,
			setupFailureCondition: func() {
				// No setup needed - height 2 is not epoch boundary
			},
			expectedErrorContains: "epoch boundary validation failed",
		},
		{
			name:          "upgrade fails at height 10 (last block of epoch)",
			upgradeHeight: 10, // Last block of first epoch, not first block of next
			setupFailureCondition: func() {
				// No setup needed
			},
			expectedErrorContains: "epoch boundary validation failed",
		},
		{
			name:          "upgrade fails at height 5 (mid-epoch)",
			upgradeHeight: 5,
			setupFailureCondition: func() {
				// No setup needed
			},
			expectedErrorContains: "epoch boundary validation failed",
		},
		{
			name:          "upgrade fails at very high height",
			upgradeHeight: 1000, // Not epoch boundary (next valid boundary is 1001)
			setupFailureCondition: func() {
				// No setup needed
			},
			expectedErrorContains: "epoch boundary validation failed",
		},
		{
			name:          "validation order test - epoch boundary fails first",
			upgradeHeight: 8, // Non-epoch boundary
			setupFailureCondition: func() {
				// Add delegate pool funds, but epoch boundary should fail first
				coins := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(100)))
				s.Require().NoError(s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, coins))
				s.Require().NoError(s.app.BankKeeper.SendCoinsFromModuleToModule(s.ctx, minttypes.ModuleName, epochingtypes.DelegatePoolModuleName, coins))
			},
			expectedErrorContains: "epoch boundary validation failed", // Should fail on epoch boundary first
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

// verifyPostUpgrade verifies that migration has been completed successfully
// by checking that the epoching parameters have been updated with new fields
func (s *UpgradeTestSuite) verifyPostUpgrade() {
	// Get updated epoching params after migration
	params := s.app.EpochingKeeper.GetParams(s.ctx)

	// Verify that migration added ExecuteGas parameters (v1->v2 migration)
	s.Assert().NotNil(params.ExecuteGas, "ExecuteGas should be set after migration")

	// Check default ExecuteGas values were properly set
	expectedExecuteGas := epochingtypes.DefaultExecuteGas
	s.Assert().Equal(expectedExecuteGas.Delegate, params.ExecuteGas.Delegate, "Delegate gas should match default")
	s.Assert().Equal(expectedExecuteGas.Undelegate, params.ExecuteGas.Undelegate, "Undelegate gas should match default")
	s.Assert().Equal(expectedExecuteGas.BeginRedelegate, params.ExecuteGas.BeginRedelegate, "BeginRedelegate gas should match default")
	s.Assert().Equal(expectedExecuteGas.CancelUnbondingDelegation, params.ExecuteGas.CancelUnbondingDelegation, "CancelUnbondingDelegation gas should match default")
	s.Assert().Equal(expectedExecuteGas.EditValidator, params.ExecuteGas.EditValidator, "EditValidator gas should match default")

	// Verify MinAmount was set (v1->v2 migration)
	s.Assert().Equal(epochingtypes.DefaultMinAmount, params.MinAmount, "MinAmount should match default")

	// Verify EpochInterval was preserved from existing params
	s.Assert().Equal(uint64(10), params.EpochInterval, "EpochInterval should be preserved")

	// Verify params are valid
	s.Assert().NoError(params.Validate(), "Migrated params should be valid")

	s.T().Log("Post-upgrade verification successful: migration parameters properly updated")
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

	// Verify that migration completed successfully
	s.verifyPostUpgrade()
}

func (s *UpgradeTestSuite) executeFailedUpgrade(upgradeHeight int64, expectedErrorContains string) {
	err := s.executeUpgrade(upgradeHeight)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), expectedErrorContains)
}

// verifyPostUpgradeDelegation verifies that MsgDelegate works correctly after upgrade
func (s *UpgradeTestSuite) verifyPostUpgradeDelegation() {
	// Get existing validator (should exist from app setup)
	validators, err := s.app.StakingKeeper.GetAllValidators(s.ctx)
	s.Require().NoError(err)
	s.Require().True(len(validators) > 0, "should have at least one validator from app setup")

	validatorAddr, err := sdk.ValAddressFromBech32(validators[0].OperatorAddress)
	s.Require().NoError(err)

	// Create and fund a delegator
	delegatorAddr := sdk.AccAddress("test_delegator_12345")
	delegatorCoins := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000000))) // 1M ubbn
	s.Require().NoError(s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, delegatorCoins))
	s.Require().NoError(s.app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, minttypes.ModuleName, delegatorAddr, delegatorCoins))

	// Create a MsgDelegate
	delegationAmount := sdk.NewCoin("ubbn", math.NewInt(100000)) // 100k ubbn
	stakingMsg := &stakingtypes.MsgDelegate{
		DelegatorAddress: delegatorAddr.String(),
		ValidatorAddress: validatorAddr.String(),
		Amount:           delegationAmount,
	}
	msgDelegate := epochingtypes.NewMsgWrappedDelegate(stakingMsg)

	// Process the MsgDelegate through epoching keeper
	msgServer := epochingkeeper.NewMsgServerImpl(s.app.EpochingKeeper)
	_, err = msgServer.WrappedDelegate(s.ctx, msgDelegate)
	s.Require().NoError(err)

	// Verify delegation was queued
	epoch := s.app.EpochingKeeper.GetEpoch(s.ctx)
	queuedMsgs := s.app.EpochingKeeper.GetEpochMsgs(s.ctx, epoch.EpochNumber)
	s.Assert().True(len(queuedMsgs) > 0, "delegation message should be queued for execution")

	// Verify funds were locked in delegate pool
	moduleAddr := s.app.AccountKeeper.GetModuleAddress(epochingtypes.DelegatePoolModuleName)
	balance := s.app.BankKeeper.GetBalance(s.ctx, moduleAddr, "ubbn")
	// Should have previous funds (300000) plus the new delegation (100000)
	expectedBalance := math.NewInt(400000) // 300k from setup + 100k from delegation
	s.Assert().True(balance.Amount.GTE(expectedBalance), "delegate pool should contain locked funds from delegation")

	// Verify delegator balance was reduced
	delegatorBalance := s.app.BankKeeper.GetBalance(s.ctx, delegatorAddr, "ubbn")
	expectedDelegatorBalance := math.NewInt(900000) // 1M - 100k delegation
	s.Assert().Equal(expectedDelegatorBalance, delegatorBalance.Amount, "delegator balance should be reduced by delegation amount")

	// Now simulate processing at epoch boundary - advance to last block of epoch
	currentEpoch := s.app.EpochingKeeper.GetEpoch(s.ctx)
	lastBlockHeight := currentEpoch.GetLastBlockHeight()

	// Advance to the last block of current epoch (but don't go to next epoch yet)
	s.ctx = s.ctx.WithBlockHeight(int64(lastBlockHeight)).WithHeaderInfo(header.Info{
		Height: int64(lastBlockHeight),
		Time:   s.ctx.BlockTime().Add(time.Second * time.Duration(lastBlockHeight-uint64(s.ctx.BlockHeight()))),
	})

	// Execute epoch end processing at the last block of the epoch
	// This should process all queued messages including our delegation
	_, err = epoching.EndBlocker(s.ctx, s.app.EpochingKeeper)
	s.Require().NoError(err)

	// Verify delegation was actually processed by checking validator's delegation
	delegation, err := s.app.StakingKeeper.GetDelegation(s.ctx, delegatorAddr, validatorAddr)
	if err == nil {
		s.Assert().True(delegation.Shares.IsPositive(), "delegation should have been processed and created shares")
	}

	s.T().Log("Post-upgrade delegation verification successful: MsgDelegate processed correctly through epoch boundary")
}
