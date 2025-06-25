package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	"github.com/test-go/testify/require"
)

func TestConsumerRegisterValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		input       types.ConsumerRegister
		expectedErr string
	}{
		{
			desc: "valid consumer",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "A valid consumer",
			},
		},
		{
			desc: "missing ConsumerId",
			input: types.ConsumerRegister{
				ConsumerId:          "",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "A valid consumer",
			},
			expectedErr: "ConsumerId must be non-empty",
		},
		{
			desc: "missing ConsumerName",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "",
				ConsumerDescription: "A valid consumer",
			},
			expectedErr: "ConsumerName must be non-empty",
		},
		{
			desc: "missing ConsumerDescription",
			input: types.ConsumerRegister{
				ConsumerId:          "c1",
				ConsumerName:        "Consumer One",
				ConsumerDescription: "",
			},
			expectedErr: "ConsumerDescription must be non-empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
