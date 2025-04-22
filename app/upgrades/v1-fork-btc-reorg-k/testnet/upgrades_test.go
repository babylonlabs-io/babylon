package testnet_test

import (
	_ "embed"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	bbn "github.com/babylonlabs-io/babylon/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/app/upgrades/v1-fork-btc-reorg-k/testnet"
)

const (
	DummyUpgradeHeight = 5
)

type ForkTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *app.BabylonApp
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(ForkTestSuite))
}

func (s *ForkTestSuite) TestFork() {
	tcs := []struct {
		title     string
		preFork   func()
		forkLogic func()
		postFork  func()
	}{
		{
			"Fork with valid BTC delegation prior to rollback",
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()
			},
		},
	}

	for _, tc := range tcs {
		s.Run(fmt.Sprintf("Case %s", tc.title), func() {
			s.SetupTest() // reset

			tc.preFork()
			tc.forkLogic()
			tc.postFork()
		})
	}
}

func (s *ForkTestSuite) SetupTest() {
	// inject the fork
	app.Forks = []upgrades.Fork{
		upgrades.Fork{
			UpgradeName:    "testing",
			UpgradeHeight:  DummyUpgradeHeight,
			BeginForkLogic: testnet.CreateForkLogic,
		},
	}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
}

func (s *ForkTestSuite) PreUpgrade() {
}

func (s *ForkTestSuite) Upgrade() {
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)

	// execute upgrade
	s.ctx = s.ctx.WithHeaderInfo(header.Info{Height: DummyUpgradeHeight, Time: s.ctx.BlockTime().Add(time.Second)}).WithBlockHeight(DummyUpgradeHeight)
	s.NotPanics(func() {
		_, err := s.app.BeginBlocker(s.ctx)
		s.Require().NoError(err)
	})
}

func (s *ForkTestSuite) PostUpgrade() {

}
