package keeper_test

import (
	"context"
	storetypes "cosmossdk.io/store/types"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	sdktestdata "github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var _ sdk.FeeTx = &mockFeeTx{}

type mockFeeTx struct {
	Msgs       []sdk.Msg
	feePayer   []byte
	feeGranter []byte
	fee        sdk.Coins
	gas        uint64
}

func (tx *mockFeeTx) GetMsgs() []sdk.Msg {
	return tx.Msgs
}

func (tx *mockFeeTx) GetMsgsV2() ([]protoreflect.ProtoMessage, error) {
	return nil, nil
}

func (tx *mockFeeTx) FeePayer() []byte   { return tx.feePayer }
func (tx *mockFeeTx) FeeGranter() []byte { return tx.feeGranter }
func (tx *mockFeeTx) GetFee() sdk.Coins  { return tx.fee }
func (tx *mockFeeTx) GetGas() uint64     { return tx.gas }

type MockAccountKeeper struct {
	FundedAddr sdk.AccAddress
}

func (m MockAccountKeeper) GetAccount(_ context.Context, addr sdk.AccAddress) sdk.AccountI {
	if m.FundedAddr != nil && addr.Equals(m.FundedAddr) {
		return &authtypes.BaseAccount{Address: addr.String()}
	}
	return nil
}
func (m MockAccountKeeper) SetAccount(_ context.Context, _ sdk.AccountI) {}
func (m MockAccountKeeper) NewAccountWithAddress(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m MockAccountKeeper) RemoveAccount(_ context.Context, _ sdk.AccountI) {}
func (m MockAccountKeeper) GetModuleAddress(_ string) sdk.AccAddress        { return sdk.AccAddress{} }
func (m MockAccountKeeper) GetParams(_ context.Context) authtypes.Params {
	return authtypes.DefaultParams()
}

func (m MockAccountKeeper) GetSequence(_ context.Context, _ sdk.AccAddress) (uint64, error) {
	return 0, nil
}
func (m MockAccountKeeper) RemoveExpiredUnorderedNonces(_ sdk.Context) error { return nil }
func (m MockAccountKeeper) TryAddUnorderedNonce(_ sdk.Context, _ []byte, _ time.Time) error {
	return nil
}
func (m MockAccountKeeper) UnorderedTransactionsEnabled() bool { return false }
func (m MockAccountKeeper) AddressCodec() address.Codec        { return nil }

func TestCheckTxAndClearIndex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	iKeeper, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)
	decorator := keeper.NewRefundTxDecorator(iKeeper, nil, nil)

	testCases := []struct {
		name     string
		setup    func(ctx sdk.Context) sdk.Tx
		expected bool
	}{
		{
			name: "if all messages are unique and refundable, the tx is refundable",
			setup: func(ctx sdk.Context) sdk.Tx {
				msg1 := types.MsgWithdrawReward{
					Address: "address",
				}
				msg2 := types.MsgWithdrawReward{
					Address: "address2",
				}
				iKeeper.IndexRefundableMsg(ctx, &msg1)
				iKeeper.IndexRefundableMsg(ctx, &msg2)
				return &mockFeeTx{Msgs: []sdk.Msg{&msg1, &msg2}}
			},
			expected: true,
		},
		{
			name: "if some messages are not refundable, the tx is not refundable",
			setup: func(ctx sdk.Context) sdk.Tx {
				msg1 := types.MsgWithdrawReward{
					Address: "address",
				}
				msg2 := types.MsgWithdrawReward{
					Address: "address2",
				}
				iKeeper.IndexRefundableMsg(ctx, &msg1)
				return &mockFeeTx{Msgs: []sdk.Msg{&msg1, &msg2}}
			},
			expected: false,
		},
		{
			name: "if some messages are duplicated, the tx is not refundable",
			setup: func(ctx sdk.Context) sdk.Tx {
				msg := types.MsgWithdrawReward{
					Address: "address",
				}
				msg2 := types.MsgWithdrawReward{
					Address: "address",
				}
				msg3 := types.MsgWithdrawReward{
					Address: "address2",
				}
				iKeeper.IndexRefundableMsg(ctx, &msg)
				iKeeper.IndexRefundableMsg(ctx, &msg2)
				iKeeper.IndexRefundableMsg(ctx, &msg3)
				return &mockFeeTx{Msgs: []sdk.Msg{&msg, &msg2, &msg3}}
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := tc.setup(ctx)
			result := decorator.CheckTxAndClearIndex(ctx, tx)

			require.Equal(t, tc.expected, result)

			// Check that all messages have been cleared from the index
			for _, msg := range tx.GetMsgs() {
				require.False(t, iKeeper.HasRefundableMsg(ctx, types.HashMsg(msg)))
			}
		})
	}
}

func TestRefundTxDecorator_AnteHandle(t *testing.T) {
	type feeInfo struct {
		feePayer   sdk.AccAddress
		feeGranter sdk.AccAddress
	}

	testCases := []struct {
		name        string
		msgs        []sdk.Msg
		feeInfo     feeInfo
		expectErr   bool
		expectedErr string
	}{
		{
			name:      "non-refundable tx passes to next",
			msgs:      []sdk.Msg{sdktestdata.NewTestMsg()},
			expectErr: false,
		},
		{
			name: "refund tx without fee granter",
			msgs: []sdk.Msg{
				&btclctypes.MsgInsertHeaders{},
			},
			feeInfo:   feeInfo{feePayer: []byte("payer"), feeGranter: nil},
			expectErr: false,
		},
		{
			name: "refund tx with matching fee granter and payer",
			msgs: []sdk.Msg{
				&btclctypes.MsgInsertHeaders{},
			},
			feeInfo:   feeInfo{feePayer: []byte("payer"), feeGranter: []byte("payer")},
			expectErr: false,
		},
		{
			name: "non-refund tx with mixed msgs and different fee granter and payer",
			msgs: []sdk.Msg{
				sdktestdata.NewTestMsg(),
				&btclctypes.MsgInsertHeaders{},
			},
			feeInfo:   feeInfo{feePayer: []byte("payer"), feeGranter: []byte("granter")},
			expectErr: false,
		},
		{
			name: "refund tx with different fee granter and payer",
			msgs: []sdk.Msg{
				&btclctypes.MsgInsertHeaders{},
			},
			feeInfo:     feeInfo{feePayer: []byte("payer"), feeGranter: []byte("granter")},
			expectErr:   true,
			expectedErr: "it is not possible to use a fee grant in a refundable transaction",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decorator := keeper.NewRefundTxDecorator(nil, MockAccountKeeper{}, nil)

			// Create a mock FeeTx
			tx := &mockFeeTx{
				Msgs:       tc.msgs,
				feePayer:   tc.feeInfo.feePayer,
				feeGranter: tc.feeInfo.feeGranter,
			}

			// Wrap in context
			ctx := sdk.Context{}.WithChainID("test-chain")
			ctx = ctx.WithGasMeter(storetypes.NewGasMeter(1e10))

			// Next handler simply returns no error
			next := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return ctx, nil
			}

			_, err := decorator.AnteHandle(ctx, tx, false, next)
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
