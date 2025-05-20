package ante_test

import (
	"math"
	"testing"

	sdktestdata "github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app/ante"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

func TestPriorityDecorator(t *testing.T) {
	tests := []struct {
		name     string
		msgs     []sdk.Msg
		initPrio int64
		expected int64
	}{
		{
			name:     "Regular tx with lower priority",
			msgs:     []sdk.Msg{},
			initPrio: math.MaxInt64,
			expected: ante.RegularTxMaxPriority,
		},
		{
			name: "Liveness tx",
			msgs: []sdk.Msg{
				&btclctypes.MsgInsertHeaders{},
			},
			initPrio: 0,
			expected: ante.LivenessTxPriority,
		},
		{
			name: "Liveness tx with many messages",
			msgs: []sdk.Msg{
				&ftypes.MsgAddFinalitySig{},
				&btclctypes.MsgInsertHeaders{},
			},
			initPrio: 50,
			expected: ante.LivenessTxPriority,
		},
		{
			name: "Mixed messages (regular and liveness-related), is not liveness-tx",
			msgs: []sdk.Msg{
				&ftypes.MsgAddFinalitySig{},
				&sdktestdata.TestMsg{},
			},
			initPrio: 50,
			expected: 50,
		},
		{
			name: "Regular tx with low priority",
			msgs: []sdk.Msg{
				&sdktestdata.TestMsg{},
			},
			initPrio: 500,
			expected: 500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := sdk.Context{}.WithPriority(tc.initPrio)
			tx := sdk.Tx(mockTx{msgs: tc.msgs})
			deco := ante.NewPriorityDecorator()

			next := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return ctx, nil
			}

			newCtx, err := deco.AnteHandle(ctx, tx, false, next)
			require.NoError(t, err)
			require.Equal(t, tc.expected, newCtx.Priority())
		})
	}
}

type mockTx struct {
	sdk.Tx
	msgs []sdk.Msg
}

func (mt mockTx) GetMsgs() []sdk.Msg {
	return mt.msgs
}
