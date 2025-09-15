package keeper_test

import (
	"testing"

	sdktestdata "github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
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

func TestCheckTxAndClearIndex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	iKeeper, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)
	decorator := keeper.NewRefundTxDecorator(iKeeper, nil)

	testCases := []struct {
		name     string
		setup    func(ctx sdk.Context) sdk.Tx
		expected bool
	}{
		{
			name: "if all messages are unique and refundable, the tx is refundable",
			setup: func(ctx sdk.Context) sdk.Tx {
				msg1 := ftypes.MsgAddFinalitySig{
					Signer: "signer1",
				}
				msg2 := ftypes.MsgAddFinalitySig{
					Signer: "signer2",
				}
				iKeeper.IncRefundableMsgCount()
				iKeeper.IncRefundableMsgCount()
				return &mockFeeTx{Msgs: []sdk.Msg{&msg1, &msg2}}
			},
			expected: true,
		},
		{
			name: "if some messages are not refundable, the tx is not refundable",
			setup: func(ctx sdk.Context) sdk.Tx {
				msg1 := ftypes.MsgAddFinalitySig{
					Signer: "signer1",
				}
				msg2 := ftypes.MsgAddFinalitySig{
					Signer: "signer2",
				}
				iKeeper.IncRefundableMsgCount()
				return &mockFeeTx{Msgs: []sdk.Msg{&msg1, &msg2}}
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := tc.setup(ctx)
			result := decorator.CheckTxAndClearIndex(ctx, tx)

			require.Equal(t, tc.expected, result)

			// Check that refundable msg count is reset to zero
			require.True(t, iKeeper.RefundableMsgCount == 0)
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
			decorator := keeper.NewRefundTxDecorator(nil, nil)

			// Create a mock FeeTx
			tx := &mockFeeTx{
				Msgs:       tc.msgs,
				feePayer:   tc.feeInfo.feePayer,
				feeGranter: tc.feeInfo.feeGranter,
			}

			// Wrap in context
			ctx := sdk.Context{}.WithChainID("test-chain")

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
