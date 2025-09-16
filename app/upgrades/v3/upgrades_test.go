package v3_test

import (
	"testing"
	"time"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
)

const (
	DummyUpgradeHeight               = 11
	expectedZoneConciergeModuleName  = "zc"
	expectedBtcStkConsumerModuleName = "btcstkconsumer"
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

func (s *UpgradeTestSuite) setupTestWithNetwork(
	permissionedIntegration bool,
	fpCount, btcActivationHeight, ibcPacketTimeoutSeconds uint32,
) {
	s.initialBtcHeight = 100
	app.Upgrades = []upgrades.Upgrade{v3.CreateUpgrade(permissionedIntegration, fpCount, btcActivationHeight, ibcPacketTimeoutSeconds)}

	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	params := s.app.BTCStakingKeeper.GetParams(s.ctx)
	params.BtcActivationHeight = uint32(s.initialBtcHeight)
	err := s.app.BTCStakingKeeper.SetParams(s.ctx, params)
	s.Require().NoError(err)
}

func (s *UpgradeTestSuite) executeUpgrade() {
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: v3.UpgradeName, Height: DummyUpgradeHeight}
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
		name                        string
		expectedMaxFPs              uint32
		expectedBtcActivationHeight uint32
		permissionedIntegration     bool
		ibcPacketTimeoutSeconds     uint32
	}{
		{
			name:                        "mainnet upgrade",
			expectedMaxFPs:              5,
			expectedBtcActivationHeight: 915000,
			permissionedIntegration:     true,
			ibcPacketTimeoutSeconds:     2419200,
		},
		{
			name:                        "testnet upgrade",
			expectedMaxFPs:              10,
			expectedBtcActivationHeight: 264773,
			permissionedIntegration:     false,
			ibcPacketTimeoutSeconds:     2419200,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.setupTestWithNetwork(
				tc.permissionedIntegration,
				tc.expectedMaxFPs, tc.expectedBtcActivationHeight, tc.ibcPacketTimeoutSeconds,
			)

			s.executeUpgrade()

			s.verifyPostUpgrade(
				tc.permissionedIntegration,
				tc.expectedMaxFPs, tc.expectedBtcActivationHeight, tc.ibcPacketTimeoutSeconds,
			)
		})
	}
}

func (s *UpgradeTestSuite) verifyPostUpgrade(
	expectedPermissionedIntegration bool,
	expectedMaxFPs, expectedBtcActivationHeight, expectedIbcPacketTimeoutSeconds uint32,
) {
	_, found := s.app.ModuleManager.Modules[v3.DeletedCapabilityStoreKey]
	s.Require().False(found, "x/capability module should be deleted")

	_, found = s.app.ModuleManager.Modules[expectedBtcStkConsumerModuleName]
	s.Require().True(found, "x/btcstkconsumer module should be found")

	_, found = s.app.ModuleManager.Modules[expectedZoneConciergeModuleName]
	s.Require().True(found, "x/zoneconcierge module should be found")

	btcStakingParams := s.app.BTCStakingKeeper.GetParams(s.ctx)
	s.Require().Equal(expectedMaxFPs, btcStakingParams.MaxFinalityProviders, "MaxFinalityProviders should match expected value")
	s.Require().Equal(expectedBtcActivationHeight, btcStakingParams.BtcActivationHeight, "BtcActivationHeight should be set to absolute height")

	btcStkConsumerParams := s.app.BTCStkConsumerKeeper.GetParams(s.ctx)
	s.Require().Equal(expectedPermissionedIntegration, btcStkConsumerParams.PermissionedIntegration,
		"IbcPacketTimeoutSeconds should be set to absolute height")

	zoneConciergeParams := s.app.ZoneConciergeKeeper.GetParams(s.ctx)
	s.Require().Equal(expectedIbcPacketTimeoutSeconds, zoneConciergeParams.IbcPacketTimeoutSeconds,
		"IbcPacketTimeoutSeconds should be set to absolute height")

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
