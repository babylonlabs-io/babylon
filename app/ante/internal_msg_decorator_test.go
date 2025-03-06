package ante_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/app/ante"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestInternalMessageDecoratorExecutionModes(t *testing.T) {
	decorator := ante.NewValidateInternalMsgDecorator()
	executionModes := []struct {
		name      string
		execMode  sdk.ExecMode
		expectErr bool
	}{
		{"prepare proposal", sdk.ExecModePrepareProposal, true},
		{"process proposal", sdk.ExecModeProcessProposal, true},
		{"recheck", sdk.ExecModeReCheck, true},
		{"vote extension", sdk.ExecModeVoteExtension, true},
		{"verify vote extension", sdk.ExecModeVerifyVoteExtension, true},
		{"finalize", sdk.ExecModeFinalize, true},
		{"deliver", sdk.ExecModeFinalize, true},
	}

	for _, tc := range executionModes {
		t.Run(tc.name, func(t *testing.T) {
			ctx := sdk.Context{}.WithExecMode(tc.execMode)
			tx := TestTx{
				msgs: []sdk.Msg{
					&ckpttypes.MsgInjectedCheckpoint{},
				},
			}
			next := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return ctx, nil
			}

			_, err := decorator.AnteHandle(ctx, tx, false, next)
			if tc.expectErr {
				require.Error(t, err, "expected an error but got none")
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
			}
		})
	}
}
