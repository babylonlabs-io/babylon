package ante_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app/ante"
	types "github.com/babylonlabs-io/babylon/app/ante/types"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestGasDecorator(t *testing.T) {
	decorator := ante.NewBypassGasDecorator()
	tests := []struct {
		name         string
		msgs         []sdk.Msg
		expectBypass bool
	}{
		{
			name: "single msg",
			msgs: []sdk.Msg{
				&ckpttypes.MsgInjectedCheckpoint{},
			},
			expectBypass: true,
		},
		{
			name: "multiple msg",
			msgs: []sdk.Msg{
				&ckpttypes.MsgInjectedCheckpoint{},
				&ckpttypes.MsgInjectedCheckpoint{},
			},
			expectBypass: false,
		},
		{
			name: "wrong type",
			msgs: []sdk.Msg{
				&ckpttypes.MsgWrappedCreateValidator{},
				&ckpttypes.MsgWrappedCreateValidator{},
			},
			expectBypass: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := sdk.Context{}
			tx := TestTx{msgs: tc.msgs}

			next := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return ctx, nil
			}

			newCtx, err := decorator.AnteHandle(ctx, tx, false, next)
			require.NoError(t, err)

			if tc.expectBypass {
				require.IsType(t, types.NewBypassGasMeter(), newCtx.BlockGasMeter(), "expected bypass gas meter")
			} else {
				require.False(t, types.NewBypassGasMeter() == newCtx.BlockGasMeter(), "expected normal gas meter")
			}
		})
	}
}
