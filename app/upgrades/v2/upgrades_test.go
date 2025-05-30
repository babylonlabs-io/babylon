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
	ratelimittypes "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"

	"github.com/babylonlabs-io/babylon/v2/app"
	v2 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v2"
	btclighttypes "github.com/babylonlabs-io/babylon/v2/x/btclightclient/types"

	"github.com/stretchr/testify/suite"
)

const DummyUpgradeHeight = 5

var usedChannels = []channeltypes.IdentifiedChannel{
	{ChannelId: "channel-0"},
	{ChannelId: "channel-1"},
	{ChannelId: "channel-5"},
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
	testCases := []struct {
		msg             string
		baseBtcHeader   *btclighttypes.BTCHeaderInfo
		preUpgrade      func()
		upgrade         func()
		postUpgrade     func()
		includeAsyncICQ bool
	}{
		{
			"Test launch software upgrade v2 with async ICQ not included",
			sample.MainnetBtcHeader854784(s.T()),
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()
				// check the reward tracker event last processed height
				lastProcessedHeight, err := s.app.IncentiveKeeper.GetRewardTrackerEventLastProcessedHeight(s.ctx)
				s.NoError(err)
				s.EqualValues(DummyUpgradeHeight-1, lastProcessedHeight)
			},
			false,
		},
		{
			"Test launch software upgrade v2 with async ICQ included",
			sample.MainnetBtcHeader854784(s.T()),
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()
				// check the reward tracker event last processed height
				lastProcessedHeight, err := s.app.IncentiveKeeper.GetRewardTrackerEventLastProcessedHeight(s.ctx)
				s.NoError(err)
				s.EqualValues(DummyUpgradeHeight-1, lastProcessedHeight)
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			s.SetupTest(tc.includeAsyncICQ) // reset

			tc.preUpgrade()
			tc.upgrade()
			tc.postUpgrade()
		})
	}
}

func (s *UpgradeTestSuite) SetupTest(includeAsyncICQ bool) {
	// add the upgrade plan
	app.Upgrades = []upgrades.Upgrade{v2.CreateUpgrade(includeAsyncICQ)}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	// create some channels to test the rate limit logic
	for _, ch := range usedChannels {
		channel := channeltypes.NewChannel(channeltypes.OPEN, channeltypes.UNORDERED, channeltypes.NewCounterparty("transfer", "channel-1"), []string{"connection-0"}, "ics20-1")
		s.app.IBCKeeper.ChannelKeeper.SetChannel(s.ctx, "transfer", ch.ChannelId, channel)
	}
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

func (s *UpgradeTestSuite) PostUpgrade() {
	// check rate limits are set
	res, err := s.app.RatelimitKeeper.AllRateLimits(s.ctx, &ratelimittypes.QueryAllRateLimitsRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(res)
	s.Require().Len(res.RateLimits, len(usedChannels))

	for _, rl := range res.RateLimits {
		s.Require().Equal(v2.DefaultDailyLimit, rl.Quota.MaxPercentRecv)
		s.Require().Equal(v2.DefaultDailyLimit, rl.Quota.MaxPercentSend)
		s.Require().Equal(v2.DailyDurationHours, rl.Quota.DurationHours)
		s.Require().Zero(rl.Flow.Inflow.Int64())
		s.Require().Zero(rl.Flow.Outflow.Int64())
	}
}
