package v5_test

import (
	mathrand "math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/math"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	epochingutils "github.com/babylonlabs-io/babylon/v4/app/upgrades/epoching"
	v5 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v5"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/suite"
)

const (
	DummyUpgradeHeight = 25 // Height 25 to ensure we're in epoch 3 (epochs: 1-10, 11-20, 21-30)
	UpgradeName        = "v5"
)

type UpgradeTestSuite struct {
	suite.Suite

	h         *testhelper.Helper
	ctx       sdk.Context
	app       *app.BabylonApp
	preModule appmodule.HasPreBlocker
	r         *mathrand.Rand

	// Test data
	epochMsgsData map[uint64][]epochingtypes.QueuedMessage
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) SetupTest() {
	s.r = mathrand.New(mathrand.NewSource(time.Now().Unix()))

	app.Upgrades = []upgrades.Upgrade{v5.Upgrade}

	// set up helper with proper genesis accounts and balances
	s.h = testhelper.NewHelper(s.T())
	s.app = s.h.App

	// Enter 1st epoch to avoid epoch 0 issue
	var err error
	s.ctx, err = s.h.ApplyEmptyBlockWithVoteExtension(s.r)
	s.NoError(err)

	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	// Initialize test data
	s.epochMsgsData = make(map[uint64][]epochingtypes.QueuedMessage)
}

func (s *UpgradeTestSuite) TestUpgradeMessageQueueCleaning() {
	testCases := []struct {
		name        string
		preUpgrade  func()
		postUpgrade func()
	}{
		{
			name: "Clear epoch messages from historical epochs",
			preUpgrade: func() {
				s.setupEpochMessages()
			},
			postUpgrade: func() {
				s.verifyEpochMessagesCleared()
			},
		},
		{
			name: "No epoch messages to clear - empty queues",
			preUpgrade: func() {
				// Progress to epoch 3 without adding messages
				s.progressToNextEpoch() // epoch 1 -> 2
				s.progressToNextEpoch() // epoch 2 -> 3
			},
			postUpgrade: func() {
				// Verify no errors occurred and current epoch is maintained
				currentEpoch := s.app.EpochingKeeper.GetEpoch(s.ctx)
				s.Equal(uint64(3), currentEpoch.EpochNumber)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset

			tc.preUpgrade()
			s.executeUpgrade()
			tc.postUpgrade()
		})
	}
}

func (s *UpgradeTestSuite) setupEpochMessages() {
	// First, add some test messages to epochs by using message server
	// We'll create messages during epoch 1 and epoch 2, then move to epoch 3

	// Start in epoch 1 and create some messages
	s.createMessagesInCurrentEpoch(3) // Add 3 messages to epoch 1

	// Progress to epoch 2
	s.progressToNextEpoch()
	s.createMessagesInCurrentEpoch(3) // Add 3 messages to epoch 2

	// Progress to epoch 3
	s.progressToNextEpoch()
	s.createMessagesInCurrentEpoch(2) // Add 2 messages to epoch 3

	// Verify we're in epoch 3
	currentEpoch := s.app.EpochingKeeper.GetEpoch(s.ctx)
	s.Equal(uint64(3), currentEpoch.EpochNumber, "Should be in epoch 3")
}

func (s *UpgradeTestSuite) verifyEpochMessagesCleared() {
	epochingKeeper := s.app.EpochingKeeper

	// Verify historical epochs (1-2) have been cleared
	for epochNum := uint64(1); epochNum <= 2; epochNum++ {
		queueLength := epochingKeeper.GetQueueLength(s.ctx, epochNum)
		s.Equal(uint64(0), queueLength, "Expected queue length 0 for cleared epoch %d", epochNum)

		msgs := epochingKeeper.GetEpochMsgs(s.ctx, epochNum)
		s.Empty(msgs, "Expected no messages for cleared epoch %d", epochNum)
	}

	// Verify current epoch (3) messages are preserved
	currentQueueLength := epochingKeeper.GetQueueLength(s.ctx, 3)
	s.Equal(uint64(2), currentQueueLength, "Expected current epoch messages to be preserved")

	currentMsgs := epochingKeeper.GetEpochMsgs(s.ctx, 3)
	s.Len(currentMsgs, 2, "Expected current epoch messages to be preserved")

	// Verify current epoch is maintained
	currentEpoch := epochingKeeper.GetEpoch(s.ctx)
	s.Equal(uint64(3), currentEpoch.EpochNumber)
}

func (s *UpgradeTestSuite) createMessagesInCurrentEpoch(count int) {
	msgSrvr := s.h.MsgSrvr

	for i := 0; i < count; i++ {
		// Use genesis account (has balance)
		delAddr := s.h.GenAccs[0].GetAddress()

		// Get a validator from the app
		validators, err := s.app.StakingKeeper.GetValidators(s.ctx, 1)
		s.NoError(err)
		s.Require().Len(validators, 1)
		valAddr := validators[0].GetOperator()

		// Create wrapped delegate message
		stakingMsg := &staking.MsgDelegate{
			DelegatorAddress: delAddr.String(),
			ValidatorAddress: valAddr,
			Amount:           sdk.NewCoin("ubbn", math.NewInt(100000)),
		}
		wrappedMsg := epochingtypes.NewMsgWrappedDelegate(stakingMsg)

		// Send message through message server
		_, err = msgSrvr.WrappedDelegate(s.ctx, wrappedMsg)
		s.NoError(err)
	}
}

func (s *UpgradeTestSuite) progressToNextEpoch() {
	epochingKeeper := s.app.EpochingKeeper

	// Use common epoch progression utility without EndBlocker
	var err error
	s.ctx, err = epochingutils.ProgressToNextEpoch(s.ctx, epochingKeeper, &epochingutils.ProgressToNextEpochOptions{
		CallEndBlocker: false,
	})
	s.NoError(err)
}

func (s *UpgradeTestSuite) executeUpgrade() {
	// inject upgrade plan at upgrade height - 1
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: UpgradeName, Height: DummyUpgradeHeight}
	err := s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan)
	s.NoError(err)

	// ensure upgrade plan exists
	actualPlan, err := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
	s.NoError(err)
	s.Equal(plan, actualPlan)

	// execute upgrade at upgrade height
	s.ctx = s.ctx.WithHeaderInfo(header.Info{
		Height: DummyUpgradeHeight,
		Time:   s.ctx.BlockTime().Add(time.Second),
	}).WithBlockHeight(DummyUpgradeHeight)

	s.NotPanics(func() {
		_, err := s.preModule.PreBlock(s.ctx)
		s.Require().NoError(err)
	})
}
