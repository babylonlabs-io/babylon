package keeper_test

import (
	"context"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"google.golang.org/protobuf/reflect/protoreflect"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

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

func TestRefundTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bankKeeper := types.NewMockBankKeeper(ctrl)
	feegrantKeeper := types.NewMockFeegrantKeeper(ctrl)
	iKeeper, ctx := keepertest.IncentiveKeeper(t, bankKeeper, nil, nil, feegrantKeeper)

	fee := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100)))
	feePayer := []byte("feePayer")
	feeGranter := []byte("feeGranter")
	expiration := ctx.BlockTime().Add(48 * time.Hour)
	period := 24 * time.Hour

	tests := []struct {
		name        string
		tx          mockFeeTx
		setupMocks  func()
		expectError bool
	}{
		{
			name: "refund to fee payer",
			tx: mockFeeTx{
				fee:      fee,
				feePayer: feePayer,
				granter:  nil,
			},
			setupMocks: func() {
				bankKeeper.EXPECT().
					SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), feePayer, fee).
					Return(nil)
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
			setupMocks: func() {
				original := &feegrant.BasicAllowance{
					SpendLimit: fee,
					Expiration: &expiration,
				}
				feegrantKeeper.EXPECT().
					GetAllowance(gomock.Any(), feeGranter, feePayer).
					Return(original, nil)

				expected := &feegrant.BasicAllowance{
					SpendLimit: fee.Add(fee...),
					Expiration: &expiration,
				}
				feegrantKeeper.EXPECT().
					UpdateAllowance(gomock.Any(), feeGranter, feePayer, expected).
					Return(nil)

				bankKeeper.EXPECT().
					SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), feeGranter, fee).
					Return(nil)
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
			setupMocks: func() {
				original := &feegrant.PeriodicAllowance{
					Basic: feegrant.BasicAllowance{
						SpendLimit: fee,
						Expiration: &expiration,
					},
					Period:           period,
					PeriodSpendLimit: fee,
					PeriodCanSpend:   fee,
					PeriodReset:      ctx.BlockTime().Add(period),
				}

				feegrantKeeper.EXPECT().
					GetAllowance(gomock.Any(), feeGranter, feePayer).
					Return(original, nil)

				expected := &feegrant.PeriodicAllowance{
					Basic: feegrant.BasicAllowance{
						SpendLimit: original.Basic.SpendLimit.Add(fee...),
						Expiration: &expiration,
					},
					Period:           period,
					PeriodSpendLimit: fee,
					PeriodCanSpend:   original.PeriodCanSpend.Add(fee...),
					PeriodReset:      original.PeriodReset,
				}
				feegrantKeeper.EXPECT().
					UpdateAllowance(gomock.Any(), feeGranter, feePayer, expected).
					Return(nil)

				bankKeeper.EXPECT().
					SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), feeGranter, fee).
					Return(nil)
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
			setupMocks: func() {
				inner := &feegrant.BasicAllowance{
					SpendLimit: fee,
					Expiration: &expiration,
				}
				anyInner, _ := codectypes.NewAnyWithValue(inner)

				original := &feegrant.AllowedMsgAllowance{
					Allowance:       anyInner,
					AllowedMessages: []string{"*"},
				}

				feegrantKeeper.EXPECT().
					GetAllowance(gomock.Any(), feeGranter, feePayer).
					Return(original, nil)

				expected := &feegrant.AllowedMsgAllowance{
					Allowance: func() *codectypes.Any {
						updated := &feegrant.BasicAllowance{
							SpendLimit: inner.SpendLimit.Add(fee...),
							Expiration: &expiration,
						}
						any, _ := codectypes.NewAnyWithValue(updated)
						return any
					}(),
					AllowedMessages: []string{"*"},
				}

				feegrantKeeper.EXPECT().
					UpdateAllowance(gomock.Any(), feeGranter, feePayer, expected).
					DoAndReturn(func(_ context.Context, _, _ []byte, allowance feegrant.FeeAllowanceI) error {
						require.IsType(t, &feegrant.AllowedMsgAllowance{}, allowance)
						return nil
					})

				bankKeeper.EXPECT().
					SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), feeGranter, fee).
					Return(nil)
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
			setupMocks: func() {
				feegrantKeeper.EXPECT().
					GetAllowance(gomock.Any(), feeGranter, feePayer).
					Return(nil, sdkerrors.ErrNotFound)

				restoredAllowance := &feegrant.BasicAllowance{
					SpendLimit: fee,
					Expiration: &expiration,
				}
				feegrantKeeper.EXPECT().
					GrantAllowance(gomock.Any(), feeGranter, feePayer, restoredAllowance)

				bankKeeper.EXPECT().
					SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), feeGranter, fee).
					Return(nil)
			},
			expectError: false,
		},
		{
			name: "zero fee, no refund",
			tx: mockFeeTx{
				fee:      sdk.NewCoins(), // no fee
				feePayer: feePayer,
				granter:  nil,
			},
			setupMocks:  func() {}, // no refund triggered
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			err := iKeeper.RefundTx(ctx, tc.tx)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
