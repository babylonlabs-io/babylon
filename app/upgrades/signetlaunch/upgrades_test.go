package signetlaunch_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
	"github.com/babylonlabs-io/babylon/x/btclightclient"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
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

func (s *UpgradeTestSuite) SetupTest() {
	// add the upgrade plan
	app.Upgrades = append(app.Upgrades, v1.Upgrade)

	// set up app
	s.app = app.Setup(s.T(), false)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	var btcHeaderZero btclighttypes.BTCHeaderInfo
	// signet btc header 0
	btcHeaderZeroStr := `{
	 	"header": "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a008f4d5fae77031e8ad22203",
	 	"hash": "00000008819873e925422c1ff0f99f7cc9bbb232af63a077a480a3633bee1ef6",
		"work": "77414720"
	}`
	buff := bytes.NewBufferString(btcHeaderZeroStr)

	err := s.app.EncodingConfig().Codec.UnmarshalJSON(buff.Bytes(), &btcHeaderZero)
	s.NoError(err)

	k := s.app.BTCLightClientKeeper
	btclightclient.InitGenesis(s.ctx, s.app.BTCLightClientKeeper, btclighttypes.GenesisState{
		Params:     k.GetParams(s.ctx),
		BtcHeaders: []*btclighttypes.BTCHeaderInfo{&btcHeaderZero},
	})
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) TestUpgrade() {
	oldHeadersLen := 0

	testCases := []struct {
		msg         string
		pre_update  func()
		update      func()
		post_update func()
	}{
		{
			"Test launch software upgrade gov prop",
			func() {
				allBtcHeaders := s.app.BTCLightClientKeeper.GetMainChainFrom(s.ctx, 0)
				oldHeadersLen = len(allBtcHeaders)
			},
			func() {
				// inject upgrade plan
				s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
				plan := upgradetypes.Plan{Name: v1.Upgrade.UpgradeName, Height: DummyUpgradeHeight}
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
					s.NoError(err)
				})
			},
			func() {
				// ensure the btc headers were added
				allBtcHeaders := s.app.BTCLightClientKeeper.GetMainChainFrom(s.ctx, 0)

				btcHeadersInserted, err := v1.LoadBTCHeadersFromData()
				s.NoError(err)
				lenHeadersInserted := len(btcHeadersInserted)

				newHeadersLen := len(allBtcHeaders)
				s.Equal(newHeadersLen, oldHeadersLen+lenHeadersInserted)

				// ensure the headers were inserted as expected
				for i, btcHeaderInserted := range btcHeadersInserted {
					btcHeaderInState := allBtcHeaders[oldHeadersLen+i]
					s.True(btcHeaderInserted.Eq(btcHeaderInState))
				}
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			s.SetupTest() // reset

			tc.pre_update()
			tc.update()
			tc.post_update()
		})
	}
}
