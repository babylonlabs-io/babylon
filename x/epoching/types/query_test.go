package types_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

type testcase struct {
	name        string
	input       types.QueuedMessage
	expected    types.QueuedMessageResponse
	expectedErr bool
}

func TestQueuedMessage_ToResponse(t *testing.T) {
	tm := time.Now()

	delMsg := &stakingtypes.MsgDelegate{
		DelegatorAddress: "delegator1",
		ValidatorAddress: "validator1",
		Amount:           sdk.NewCoin("ubbn", math.NewInt(1000000)),
	}

	undelMsg := &stakingtypes.MsgUndelegate{
		DelegatorAddress: "delegator1",
		ValidatorAddress: "validator1",
		Amount:           sdk.NewCoin("ubbn", math.NewInt(1000000)),
	}

	dmsg, dmsgr := makeQueuedMessage(delMsg, "MsgDelegate", tm)
	undmsg, undmsgr := makeQueuedMessage(undelMsg, "MsgUndelegate", tm)

	testcases := []testcase{
		{
			name:        "MsgDelegate",
			input:       dmsg,
			expected:    dmsgr,
			expectedErr: false,
		},
		{
			name:        "MsgUnDelegate",
			input:       undmsg,
			expected:    undmsgr,
			expectedErr: false,
		},
		{
			name: "unknown message type",
			input: types.QueuedMessage{
				Msg: &types.QueuedMessage_MsgUpdateParams{
					MsgUpdateParams: &stakingtypes.MsgUpdateParams{},
				},
			},
			expected: types.QueuedMessageResponse{
				EnrichedMsg: &types.EnrichedMsg{Type: "Unknown"},
			},
			expectedErr: true,
		},
		{
			name: "nil",
			input: types.QueuedMessage{
				Msg: nil,
			},
			expected: types.QueuedMessageResponse{
				EnrichedMsg: &types.EnrichedMsg{Type: "Unknown"},
			},
			expectedErr: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedErr && tc.name == "unknown message type" {
				resp := tc.input.ToResponse()
				require.Equal(t, "Unknown", resp.EnrichedMsg.Type, "expected unrecognized message to return 'Unknown'")
			} else if tc.expectedErr && tc.name == "nil" {
				require.Panics(t, func() {
					_ = tc.input.ToResponse()
				}, "panic: : invalid message type of a QueuedMessage")
				return
			}
			resp := tc.input.ToResponse()
			require.Equal(t, tc.expected.EnrichedMsg.Type, resp.EnrichedMsg.Type)
			require.Equal(t, tc.expected.EnrichedMsg.Amount, resp.EnrichedMsg.Amount)
			require.Equal(t, tc.expected.EnrichedMsg.Delegator, resp.EnrichedMsg.Delegator)
			require.Equal(t, tc.expected.TxId, resp.TxId)
			require.Equal(t, tc.expected.MsgId, resp.MsgId)
		})
	}
}

func makeQueuedMessage(msg sdk.Msg, msgType string, time time.Time) (types.QueuedMessage, types.QueuedMessageResponse) {
	qm := types.QueuedMessage{
		TxId:        []byte("tx1"),
		MsgId:       []byte("msgid1"),
		BlockHeight: 5,
		BlockTime:   &time,
	}

	var amount, delegator, validator string

	switch m := msg.(type) {
	case *stakingtypes.MsgDelegate:
		qm.Msg = &types.QueuedMessage_MsgDelegate{MsgDelegate: m}
		amount = m.Amount.Amount.String()
		delegator = m.DelegatorAddress
		validator = m.ValidatorAddress
	case *stakingtypes.MsgUndelegate:
		qm.Msg = &types.QueuedMessage_MsgUndelegate{MsgUndelegate: m}
		amount = m.Amount.Amount.String()
		delegator = m.DelegatorAddress
		validator = m.ValidatorAddress
	default:
		fmt.Errorf("unrecognised message type: %T\n", msg)
	}

	qmr := types.QueuedMessageResponse{
		TxId:        hex.EncodeToString(qm.TxId),
		MsgId:       hex.EncodeToString(qm.MsgId),
		BlockHeight: 5,
		BlockTime:   &time,
		Msg:         msg.String(),
		EnrichedMsg: &types.EnrichedMsg{
			Type:      msgType,
			Amount:    amount + "ubbn",
			Delegator: delegator,
			Validator: validator,
		},
	}

	return qm, qmr
}
