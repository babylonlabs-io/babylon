package types_test

import (
	"encoding/hex"
	"testing"

	"github.com/cosmos/gogoproto/proto"

	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestQueuedMessage_ToResponse(t *testing.T) {
	msgDelegate := &stakingtypes.MsgDelegate{}
	msgUndelegate := &stakingtypes.MsgUndelegate{}
	msgRedelegate := &stakingtypes.MsgBeginRedelegate{}
	msgCancelUnbonding := &stakingtypes.MsgCancelUnbondingDelegation{}
	msgCreateValidator := &stakingtypes.MsgCreateValidator{}
	msgBeginRedelegate := &stakingtypes.MsgBeginRedelegate{}
	msgEditValidator := &stakingtypes.MsgEditValidator{}

	testcases := []struct {
		name                string
		inputQueuedMessage  types.QueuedMessage
		expectQueuedMsgType string
		expectPanic         bool
	}{
		{
			name: "MsgDelegate",
			inputQueuedMessage: types.QueuedMessage{
				TxId:  []byte("tx1"),
				MsgId: []byte("msg1"),
				Msg:   &types.QueuedMessage_MsgDelegate{MsgDelegate: msgDelegate},
			},
			expectQueuedMsgType: proto.MessageName(msgDelegate),
		},
		{
			name: "MsgUndelegate",
			inputQueuedMessage: types.QueuedMessage{
				TxId:  []byte("tx2"),
				MsgId: []byte("msg2"),
				Msg:   &types.QueuedMessage_MsgUndelegate{MsgUndelegate: msgUndelegate},
			},
			expectQueuedMsgType: proto.MessageName(msgUndelegate),
		},
		{
			name: "MsgBeginRedelegate",
			inputQueuedMessage: types.QueuedMessage{
				TxId:  []byte("tx3"),
				MsgId: []byte("msg3"),
				Msg:   &types.QueuedMessage_MsgBeginRedelegate{MsgBeginRedelegate: msgRedelegate},
			},
			expectQueuedMsgType: proto.MessageName(msgBeginRedelegate),
		},
		{
			name: "MsgCancelUnbondingDelegation",
			inputQueuedMessage: types.QueuedMessage{
				TxId:  []byte("tx4"),
				MsgId: []byte("msg4"),
				Msg:   &types.QueuedMessage_MsgCancelUnbondingDelegation{MsgCancelUnbondingDelegation: msgCancelUnbonding},
			},
			expectQueuedMsgType: proto.MessageName(msgCancelUnbonding),
		},
		{
			name: "MsgCreateValidator",
			inputQueuedMessage: types.QueuedMessage{
				TxId:  []byte("tx5"),
				MsgId: []byte("msg5"),
				Msg:   &types.QueuedMessage_MsgCreateValidator{MsgCreateValidator: msgCreateValidator},
			},
			expectQueuedMsgType: proto.MessageName(msgCreateValidator),
		},
		{
			name: "MsgEditValidator",
			inputQueuedMessage: types.QueuedMessage{
				TxId:  []byte("tx6"),
				MsgId: []byte("msg6"),
				Msg:   &types.QueuedMessage_MsgEditValidator{MsgEditValidator: msgEditValidator},
			},
			expectQueuedMsgType: proto.MessageName(msgEditValidator),
		},
		{
			name: "nil message",
			inputQueuedMessage: types.QueuedMessage{
				Msg: nil,
			},
			expectPanic: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				require.Panics(t, func() {
					_ = tc.inputQueuedMessage.ToResponse()
				})
				return
			}

			resp := tc.inputQueuedMessage.ToResponse()

			require.NotNil(t, resp)
			require.Equal(t, tc.expectQueuedMsgType, resp.MsgType)
			require.Equal(t, hex.EncodeToString(tc.inputQueuedMessage.TxId), resp.TxId)
			require.Equal(t, hex.EncodeToString(tc.inputQueuedMessage.MsgId), resp.MsgId)
			require.Equal(t, tc.inputQueuedMessage.UnwrapToSdkMsg().String(), resp.Msg)
		})
	}
}
