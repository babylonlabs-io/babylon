package testnet_test

import (
	_ "embed"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	testutil "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/app/upgrades/v1-fork-btc-reorg-k/testnet"
	bskeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
)

const (
	DummyForkHeight       = 5
	BtcRollbackHeightFrom = 10
	BtcRollbackHeightTo   = 25
	StkAmount             = 100000
)

type ForkTestSuite struct {
	suite.Suite

	r            *rand.Rand
	ctx          sdk.Context
	app          *app.BabylonApp
	MsgSvrBtcStk bstypes.MsgServer
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
	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	// inject the fork
	app.Forks = []upgrades.Fork{
		upgrades.Fork{
			UpgradeName:    "testing",
			UpgradeHeight:  DummyForkHeight,
			BeginForkLogic: testnet.CreateForkLogic,
		},
	}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})

	// set up msg servers
	s.MsgSvrBtcStk = bskeeper.NewMsgServerImpl(s.app.BTCStakingKeeper)
}

func (s *ForkTestSuite) PreUpgrade() {
	fp, fpSk, err := datagen.GenRandomFinalityProviderWithSk(s.r)
	s.NoError(err)

	fpPk := fpSk.PubKey()
	msg := &bstypes.MsgCreateFinalityProvider{
		Addr:        fp.Addr,
		Description: fp.Description,
		Commission: bstypes.NewCommissionRates(
			*fp.Commission,
			fp.CommissionInfo.MaxRate,
			fp.CommissionInfo.MaxChangeRate,
		),
		BtcPk: fp.BtcPk,
		Pop:   fp.Pop,
	}

	_, err = s.MsgSvrBtcStk.CreateFinalityProvider(s.ctx, msg)
	s.NoError(err)

	delSK, _, err := datagen.GenRandomBTCKeyPair(s.r)
	s.NoError(err)

	stakingTxHash, _, _, _, _, _, err := testutil.CreateDelegationWithBtcBlockHeight(
		s.r,
		s.ctx,
		s.T(),
		&s.app.BTCStakingKeeper,
		s.app.BTCLightClientKeeper.GetBTCNet(),
		delSK,
		[]*btcec.PublicKey{fpPk},
		StkAmount, // stk value
		1000,      // stk time
		0,
		0,
		false, // no preapproval
		false,
		BtcRollbackHeightFrom-1,
		BtcRollbackHeightFrom,
	)
	s.NoError(err)
	s.NotNil(stakingTxHash)

	newDc := ftypes.NewVotingPowerDistCache()

	fpDstInf := ftypes.NewFinalityProviderDistInfo(fp)
	fpDstInf.AddBondedSats(StkAmount)

	newDc.AddFinalityProviderDistInfo(fpDstInf)

	s.app.FinalityKeeper.RecordVpAndDistCacheForHeight(s.ctx, newDc, DummyForkHeight-1)
}

func (s *ForkTestSuite) Upgrade() {
	s.app.BTCStakingKeeper.SetLargestBtcReorg(s.ctx, bstypes.LargestBtcReOrg{
		BlockDiff: 10,
		RollbackFrom: &types.BTCHeaderInfo{
			Height: BtcRollbackHeightFrom,
		},
		RollbackTo: &types.BTCHeaderInfo{
			Height: BtcRollbackHeightTo,
		},
	})
	s.ctx = s.ctx.WithBlockHeight(DummyForkHeight - 1)

	// execute upgrade
	s.ctx = s.ctx.WithHeaderInfo(header.Info{Height: DummyForkHeight, Time: s.ctx.BlockTime().Add(time.Second)}).WithBlockHeight(DummyForkHeight)
	s.NotPanics(func() {
		_, err := s.app.BeginBlocker(s.ctx)
		s.Require().NoError(err)
	})
}

func (s *ForkTestSuite) PostUpgrade() {

}
