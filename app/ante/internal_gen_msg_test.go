package ante_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/app/ante"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestInjectedMsg(t *testing.T) {
	tests := []struct {
		name      string
		msgs      []sdk.Msg
		expectRes bool
	}{
		{
			name: "multiple messages",
			msgs: []sdk.Msg{
				&ckpttypes.EventCheckpointFinalized{},
				&ckpttypes.EventCheckpointFinalized{},
			},
			expectRes: false,
		},
		{
			name: "single injected message",
			msgs: []sdk.Msg{
				&ckpttypes.MsgInjectedCheckpoint{},
			},
			expectRes: true,
		},
		{
			name: "single non-injected message",
			msgs: []sdk.Msg{
				&ckpttypes.EventCheckpointFinalized{},
			},
			expectRes: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ante.SingleInjectedMsg(tc.msgs)
			require.Equal(t, tc.expectRes, result, "unexpected result for test case: %s", tc.name)
		})
	}
}
