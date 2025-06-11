package keeper_test

import (
	"testing"

	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var _ sdk.Tx = &TestTx{}

type TestTx struct {
	Msgs []sdk.Msg
}

func (tx *TestTx) GetMsgs() []sdk.Msg {
	return tx.Msgs
}

func (tx *TestTx) GetMsgsV2() ([]protoreflect.ProtoMessage, error) {
	return nil, nil
}

func TestCheckTxAndClearIndex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	iKeeper, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil, nil)
	decorator := keeper.NewRefundTxDecorator(iKeeper)

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
				return &TestTx{Msgs: []sdk.Msg{&msg1, &msg2}}
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
				return &TestTx{Msgs: []sdk.Msg{&msg1, &msg2}}
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
				return &TestTx{Msgs: []sdk.Msg{&msg, &msg2, &msg3}}
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
