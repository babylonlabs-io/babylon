package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := keepertest.BTCStkConsumerKeeper(t)
	params := types.DefaultParams()

	require.NoError(t, k.SetParams(ctx, params))
	require.EqualValues(t, params, k.GetParams(ctx))
}

func TestGetMaxMultiStakedFps(t *testing.T) {
	k, ctx := keepertest.BTCStkConsumerKeeper(t)

	// Test default value
	expectedDefault := uint32(2)
	require.Equal(t, expectedDefault, k.GetMaxMultiStakedFps(ctx))

	// Test setting custom value
	params := types.Params{
		PermissionedIntegration: false,
		MaxMultiStakedFps:       5,
	}
	require.NoError(t, k.SetParams(ctx, params))
	require.Equal(t, uint32(5), k.GetMaxMultiStakedFps(ctx))
}

func TestParamsValidation(t *testing.T) {
	tests := []struct {
		name    string
		params  types.Params
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid params with default values",
			params: types.Params{
				PermissionedIntegration: false,
				MaxMultiStakedFps:       2,
			},
			wantErr: false,
		},
		{
			name: "valid params with minimum allowed value",
			params: types.Params{
				PermissionedIntegration: true,
				MaxMultiStakedFps:       4,
			},
			wantErr: false,
		},
		{
			name: "invalid params with MaxMultiStakedFps too low (1)",
			params: types.Params{
				PermissionedIntegration: false,
				MaxMultiStakedFps:       1,
			},
			wantErr: true,
			errMsg:  types.ErrInvalidMaxMultiStakedFps.Error(),
		},
		{
			name: "invalid params with MaxMultiStakedFps zero",
			params: types.Params{
				PermissionedIntegration: false,
				MaxMultiStakedFps:       0,
			},
			wantErr: true,
			errMsg:  types.ErrInvalidMaxMultiStakedFps.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, types.ErrInvalidMaxMultiStakedFps)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
