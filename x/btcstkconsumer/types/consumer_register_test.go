package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	"github.com/stretchr/testify/require"
)

func TestConsumerRegisterValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		input       types.ConsumerRegister
		expectedErr error
	}{
		{
			desc: "valid consumer",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "A valid consumer",
				MaxMultiStakedFps:   2,
			},
		},
		{
			desc: "missing ConsumerId",
			input: types.ConsumerRegister{
				ConsumerId:          "",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "A valid consumer",
				MaxMultiStakedFps:   2,
			},
			expectedErr: types.ErrEmptyConsumerId,
		},
		{
			desc: "missing ConsumerName",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "",
				ConsumerDescription: "A valid consumer",
				MaxMultiStakedFps:   2,
			},
			expectedErr: types.ErrEmptyConsumerName,
		},
		{
			desc: "missing ConsumerDescription",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "",
				MaxMultiStakedFps:   2,
			},
			expectedErr: types.ErrEmptyConsumerDescription,
		},
		{
			desc: "MaxMultiStakedFps too small (0)",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "A valid consumer",
				MaxMultiStakedFps:   0,
			},
			expectedErr: types.ErrInvalidMaxMultiStakedFps,
		},
		{
			desc: "MaxMultiStakedFps too small (1)",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "A valid consumer",
				MaxMultiStakedFps:   1,
			},
			expectedErr: types.ErrInvalidMaxMultiStakedFps,
		},
		{
			desc: "valid MaxMultiStakedFps (3)",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "A valid consumer",
				MaxMultiStakedFps:   3,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
			}
		})
	}
}
