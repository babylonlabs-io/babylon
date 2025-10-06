package keeper_test

import (
	"testing"
	"time"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RefundTxTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *app.BabylonApp
}

func TestKeeperSuite(t *testing.T) {
	suite.Run(t, new(RefundTxTestSuite))
}

func (s *RefundTxTestSuite) TestRefundTx() {
	var (
		fee        = sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100)))
		feePayer   = []byte("feePayer")
		feeGranter = []byte("feeGranter")
		zeroCoins  = sdk.NewCoins()
	)

	tests := []struct {
		name        string
		tx          sdk.FeeTx
		preRefund   func()
		postRefund  func()
		expectError bool
	}{
		{
			name: "refund to fee payer",
			tx: &mockFeeTx{
				fee:        fee,
				feePayer:   feePayer,
				feeGranter: nil,
			},
			preRefund: func() {
				// expect fee payer to have 0 balance before refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feePayer)
				s.Equal(zeroCoins, balance)
			},
			postRefund: func() {
				// expect fee payer to have been refunded refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feePayer)
				s.Equal(fee, balance)
			},
			expectError: false,
		},
		{
			name: "refund to fee granter",
			tx: &mockFeeTx{
				fee:        fee,
				feePayer:   feePayer,
				feeGranter: feeGranter,
			},
			preRefund: func() {
				// expect fee granter to have 0 balance before refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(zeroCoins, balance)

				_, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, feeGranter, feePayer)
				s.True(errorsmod.IsOf(err, sdkerrors.ErrNotFound))
			},
			postRefund: func() {
				// expect not to restore the allowance
				_, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, feeGranter, feePayer)
				s.True(errorsmod.IsOf(err, sdkerrors.ErrNotFound))

				// expect fee granter to have been refunded refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(fee, balance)
			},
			expectError: false,
		},
		{
			name: "zero fee, no refund",
			tx: &mockFeeTx{
				fee:        zeroCoins, // no fee
				feePayer:   feePayer,
				feeGranter: nil,
			},
			preRefund: func() {
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feePayer)
				s.Equal(zeroCoins, balance)
			},
			postRefund: func() {
				// no refund triggered
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feePayer)
				s.Equal(zeroCoins, balance)
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			s.SetupTest(t)
			tc.preRefund()
			err := s.app.IncentiveKeeper.RefundTx(s.ctx, tc.tx)
			tc.postRefund()
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func (s *RefundTxTestSuite) SetupTest(t *testing.T) {
	// set up app and keepers
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.app.FeeGrantKeeper = s.app.FeeGrantKeeper.SetBankKeeper(s.app.BankKeeper)
	s.app.IncentiveKeeper = keeper.NewKeeper(
		s.app.AppCodec(),
		runtime.NewKVStoreService(storetypes.NewKVStoreKey(types.StoreKey)),
		s.app.BankKeeper,
		s.app.AccountKeeper,
		&s.app.EpochingKeeper,
		appparams.AccGov.String(),
		authtypes.FeeCollectorName,
	)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})

	// send some funds to fee collector to refund the payer
	coins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100)))
	err := s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, coins)
	s.NoError(err)
	err = s.app.BankKeeper.SendCoinsFromModuleToModule(s.ctx, minttypes.ModuleName, authtypes.FeeCollectorName, coins)
	s.NoError(err)
}
