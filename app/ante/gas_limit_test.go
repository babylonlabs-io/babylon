package ante_test

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/babylonlabs-io/babylon/v3/app/ante"

	"github.com/stretchr/testify/require"
)

func TestGasLimitDecorator(t *testing.T) {
	testCases := []struct {
		name        string
		gasWanted   uint64
		isCheckTx   bool
		simulate    bool
		expectError bool
	}{
		{
			name:        "Valid gas limit",
			gasWanted:   5_000_000,
			isCheckTx:   true,
			simulate:    false,
			expectError: false,
		},
		{
			name:        "Exceeds gas limit in CheckTx",
			gasWanted:   12_000_000,
			isCheckTx:   true,
			simulate:    false,
			expectError: true,
		},
		{
			name:        "Exceeds gas limit in Simulate mode (should not fail)",
			gasWanted:   12_000_000,
			isCheckTx:   true,
			simulate:    true,
			expectError: false,
		},
		{
			name:        "Exceeds gas limit in DeliverTx (should not fail)",
			gasWanted:   12_000_000,
			isCheckTx:   false,
			simulate:    false,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := sdk.NewContext(nil, cmtproto.Header{}, tc.isCheckTx, nil)
			decorator := ante.NewGasLimitDecorator(ante.NewDefaultMempoolOptions())

			tx := &mockFeeTx{gasWanted: tc.gasWanted}
			_, err := decorator.AnteHandle(ctx, tx, tc.simulate, func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return ctx, nil
			})

			if tc.expectError {
				require.Error(t, err)
				require.True(t, sdkerrors.ErrOutOfGas.Is(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type mockFeeTx struct {
	sdk.FeeTx
	gasWanted uint64
}

func (mft *mockFeeTx) GetGas() uint64 {
	return mft.gasWanted
}
