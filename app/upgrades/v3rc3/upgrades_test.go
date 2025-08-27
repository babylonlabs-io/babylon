package v3rc3_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/x/upgrade"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stretchr/testify/suite"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	v3rc3 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc3"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/stretchr/testify/require"
)

const (
	DummyUpgradeHeight = 5
)

func TestGetLargestBtcReorg(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	headerInfo1 := datagen.GenRandomBTCHeaderInfo(r)
	headerInfo1.Height = 100

	headerInfo2 := datagen.GenRandomBTCHeaderInfo(r)
	headerInfo1.Height = 200

	tcs := []struct {
		name               string
		largestBtcReorg    btcstktypes.LargestBtcReOrg
		oldLargestBtcReorg btcstktypes.LargestBtcReOrg
		err                error
		errOld             error
		expectedResult     *btcstktypes.LargestBtcReOrg
	}{
		{
			name: "both valid - choose largest diff",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    10,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    20,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
			err:    nil,
			errOld: nil,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    20,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
		},
		{
			name: "both valid - current is larger",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    30,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    20,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
			err:    nil,
			errOld: nil,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    30,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
		},
		{
			name: "only current valid",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    15,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{},
			err:                nil,
			errOld:             collections.ErrNotFound,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    15,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
		},
		{
			name:            "only old valid",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    25,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
			err:    collections.ErrNotFound,
			errOld: nil,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    25,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
		},
		{
			name:               "neither valid",
			largestBtcReorg:    btcstktypes.LargestBtcReOrg{},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{},
			err:                collections.ErrNotFound,
			errOld:             collections.ErrNotFound,
			expectedResult:     nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actual := v3rc3.GetLargestBtcReorg(tc.largestBtcReorg, tc.oldLargestBtcReorg, tc.err, tc.errOld)
			if tc.expectedResult == nil {
				require.Nil(t, actual)
				return
			}
			require.NotNil(t, actual)
			require.Equal(t, tc.expectedResult.BlockDiff, actual.BlockDiff)
			require.Equal(t, tc.expectedResult.RollbackFrom, actual.RollbackFrom)
			require.Equal(t, tc.expectedResult.RollbackTo, actual.RollbackTo)
		})
	}
}

type UpgradeTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	app       *app.BabylonApp
	preModule appmodule.HasPreBlocker
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) TestUpgrade() {
	tcs := []struct {
		msg         string
		preUpgrade  func()
		upgrade     func()
		postUpgrade func()
	}{
		{
			"Test upgrade v3rc3 with duplicated fp addr and largest btc reorg in prefix 13",
			s.PreUpgrade,
			s.Upgrade,
			s.PostUpgrade,
		},
		{
			"Test upgrade v3rc3 witout duplicated fp addr and without largest btc reorg in any prefix",
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()
			},
		},
	}

	for _, tc := range tcs {
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
	app.Upgrades = []upgrades.Upgrade{v3rc3.Upgrade}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())
}

func (s *UpgradeTestSuite) PreUpgrade() {}

func (s *UpgradeTestSuite) Upgrade() {
	// inject upgrade plan
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: v3rc3.UpgradeName, Height: DummyUpgradeHeight}
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

func (s *UpgradeTestSuite) PostUpgrade() {

}
