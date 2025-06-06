package keeper_test

import (
	"testing"
	"time"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"

	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/suite"
)

type RefundTxTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *app.BabylonApp
}

var _ sdk.FeeTx = mockFeeTx{}

// mockFeeTx implements the sdk.FeeTx interface for testing
type mockFeeTx struct {
	fee      sdk.Coins
	feePayer []byte
	granter  []byte
}

func (tx mockFeeTx) GetFee() sdk.Coins {
	return tx.fee
}

func (tx mockFeeTx) FeePayer() []byte {
	return tx.feePayer
}

func (tx mockFeeTx) FeeGranter() []byte {
	return tx.granter
}

// Additional methods required by sdk.FeeTx interface
func (tx mockFeeTx) GetGas() uint64     { return 0 }
func (tx mockFeeTx) GetMsgs() []sdk.Msg { return nil }
func (tx mockFeeTx) GetMsgsV2() ([]protoreflect.ProtoMessage, error) {
	return nil, nil
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(RefundTxTestSuite))
}

func (s *RefundTxTestSuite) TestRefundTx() {
	var (
		fee        = sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100)))
		feePayer   = []byte("feePayer")
		feeGranter = []byte("feeGranter")
		period     = 24 * time.Hour
		zeroCoins  = sdk.NewCoins()
	)

	tests := []struct {
		name        string
		tx          mockFeeTx
		preRefund   func()
		postRefund  func()
		expectError bool
	}{
		{
			name: "refund to fee payer",
			tx: mockFeeTx{
				fee:      fee,
				feePayer: feePayer,
				granter:  nil,
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
			name: "refund to fee granter with BasicAllowance",
			tx: mockFeeTx{
				fee:      fee,
				feePayer: feePayer,
				granter:  feeGranter,
			},
			preRefund: func() {
				// expect fee granter to have 0 balance before refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(zeroCoins, balance)

				// set allowance
				expiration := s.ctx.BlockHeader().Time.Add(48 * time.Hour)
				original := &feegrant.BasicAllowance{
					SpendLimit: fee,
					Expiration: &expiration,
				}
				s.NoError(original.ValidateBasic())
				err := s.app.FeeGrantKeeper.GrantAllowance(s.ctx, feeGranter, feePayer, original)
				s.NoError(err)
			},
			postRefund: func() {
				expiration := s.ctx.BlockHeader().Time.Add(48 * time.Hour)
				expected := &feegrant.BasicAllowance{
					SpendLimit: fee.Add(fee...), // the original + the refund
					Expiration: &expiration,
				}

				updatedGrant, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, feeGranter, feePayer)
				s.NoError(err)
				s.Equal(expected, updatedGrant)

				// expect fee granter to have been refunded refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(fee, balance)
			},
			expectError: false,
		},
		{
			name: "refund to fee granter with PeriodicAllowance",
			tx: mockFeeTx{
				fee:      fee,
				feePayer: feePayer,
				granter:  feeGranter,
			},
			preRefund: func() {
				// expect fee granter to have 0 balance before refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(zeroCoins, balance)

				// set PeriodicAllowance
				expiration := s.ctx.BlockHeader().Time.Add(48 * time.Hour)
				original := &feegrant.PeriodicAllowance{
					Basic: feegrant.BasicAllowance{
						SpendLimit: fee,
						Expiration: &expiration,
					},
					Period:           period,
					PeriodSpendLimit: fee,
					PeriodCanSpend:   fee,
					PeriodReset:      s.ctx.BlockTime().Add(period),
				}
				s.NoError(original.ValidateBasic())

				err := s.app.FeeGrantKeeper.GrantAllowance(s.ctx, feeGranter, feePayer, original)
				s.NoError(err)
			},
			postRefund: func() {
				expiration := s.ctx.BlockHeader().Time.Add(48 * time.Hour)
				expected := &feegrant.PeriodicAllowance{
					Basic: feegrant.BasicAllowance{
						SpendLimit: fee.Add(fee...),
						Expiration: &expiration,
					},
					Period:           period,
					PeriodSpendLimit: fee,
					PeriodCanSpend:   fee.Add(fee...),
					PeriodReset:      s.ctx.BlockTime().Add(period),
				}

				updatedGrant, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, feeGranter, feePayer)
				s.NoError(err)
				s.Equal(expected, updatedGrant)

				// expect fee granter to have been refunded refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(fee, balance)
			},
			expectError: false,
		},
		{
			name: "refund to fee granter with AllowedMsgAllowance",
			tx: mockFeeTx{
				fee:      fee,
				feePayer: feePayer,
				granter:  feeGranter,
			},
			preRefund: func() {
				// expect fee granter to have 0 balance before refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(zeroCoins, balance)

				expiration := s.ctx.BlockHeader().Time.Add(48 * time.Hour)
				inner := &feegrant.BasicAllowance{
					SpendLimit: fee,
					Expiration: &expiration,
				}
				anyInner, err := codectypes.NewAnyWithValue(inner)
				s.NoError(err)
				original := &feegrant.AllowedMsgAllowance{
					Allowance:       anyInner,
					AllowedMessages: []string{"*"},
				}
				s.NoError(original.ValidateBasic())

				err = s.app.FeeGrantKeeper.GrantAllowance(s.ctx, feeGranter, feePayer, original)
				s.NoError(err)
			},
			postRefund: func() {
				expiration := s.ctx.BlockHeader().Time.Add(48 * time.Hour)
				expInner := &feegrant.BasicAllowance{
					SpendLimit: fee.Add(fee...),
					Expiration: &expiration,
				}
				anyInner, err := codectypes.NewAnyWithValue(expInner)
				s.NoError(err)

				expected := &feegrant.AllowedMsgAllowance{
					Allowance:       anyInner,
					AllowedMessages: []string{"*"},
				}

				updatedGrant, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, feeGranter, feePayer)
				s.NoError(err)
				s.Equal(expected, updatedGrant)

				// expect fee granter to have been refunded refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(fee, balance)
			},
			expectError: false,
		},
		{
			name: "refund to fee granter with missing allowance (create new)",
			tx: mockFeeTx{
				fee:      fee,
				feePayer: feePayer,
				granter:  feeGranter,
			},
			preRefund: func() {
				// expect fee granter to have 0 balance before refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(zeroCoins, balance)

				_, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, feeGranter, feePayer)
				s.True(errorsmod.IsOf(err, sdkerrors.ErrNotFound))
			},
			postRefund: func() {
				expiration := s.ctx.BlockHeader().Time.Add(48 * time.Hour)
				expected := &feegrant.BasicAllowance{
					SpendLimit: fee,
					Expiration: &expiration,
				}
				restoredGrant, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, feeGranter, feePayer)
				s.NoError(err)
				s.Equal(expected, restoredGrant)

				// expect fee granter to have been refunded refund
				balance := s.app.BankKeeper.GetAllBalances(s.ctx, feeGranter)
				s.Equal(fee, balance)
			},
			expectError: false,
		},
		{
			name: "zero fee, no refund",
			tx: mockFeeTx{
				fee:      zeroCoins, // no fee
				feePayer: feePayer,
				granter:  nil,
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
		s.app.FeeGrantKeeper,
		appparams.AccGov.String(),
		authtypes.FeeCollectorName,
	)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})

	// send some funds to fee collector to refund the payer
	coins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100)))
	err := s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, coins)
	s.NoError(err)
	s.app.BankKeeper.SendCoinsFromModuleToModule(s.ctx, minttypes.ModuleName, authtypes.FeeCollectorName, coins)
}
