package types_test

import (
	"fmt"
	"testing"

	"cosmossdk.io/math"
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

func TestMsgRegisterConsumerValidateBasic(t *testing.T) {
	validCommission := math.LegacyNewDecWithPrec(5, 1) // 0.5

	testCases := []struct {
		name     string
		msg      *types.MsgRegisterConsumer
		expected error
	}{
		{
			name: "valid message",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "Test Description",
				BabylonCommission:   validCommission,
			},
			expected: nil,
		},
		{
			name: "empty consumer ID",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "Test Description",
				BabylonCommission:   validCommission,
			},
			expected: fmt.Errorf("ConsumerId must be non-empty"),
		},
		{
			name: "empty consumer name",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "",
				ConsumerDescription: "Test Description",
				BabylonCommission:   validCommission,
			},
			expected: fmt.Errorf("ConsumerName must be non-empty"),
		},
		{
			name: "empty consumer description",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "",
				BabylonCommission:   validCommission,
			},
			expected: fmt.Errorf("ConsumerDescription must be non-empty"),
		},
		{
			name: "negative babylon commission",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "Test Description",
				BabylonCommission:   math.LegacyNewDecWithPrec(-1, 1), // -0.1
			},
			expected: fmt.Errorf("babylon commission cannot be negative"),
		},
		{
			name: "babylon commission greater than 1.0",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "Test Description",
				BabylonCommission:   math.LegacyNewDecWithPrec(15, 1), // 1.5
			},
			expected: fmt.Errorf("babylon commission cannot be greater than 1.0"),
		},
		{
			name: "babylon commission exactly 0.0",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "Test Description",
				BabylonCommission:   math.LegacyZeroDec(),
			},
			expected: nil,
		},
		{
			name: "babylon commission exactly 1.0",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "Test Description",
				BabylonCommission:   math.LegacyOneDec(),
			},
			expected: nil,
		},
		{
			name: "babylon commission with high precision",
			msg: &types.MsgRegisterConsumer{
				Signer:              "babylon1validaddress",
				ConsumerId:          "consumer-123",
				ConsumerName:        "Test Consumer",
				ConsumerDescription: "Test Description",
				BabylonCommission:   math.LegacyNewDecWithPrec(123456789012345678, 18),
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expected != nil {
				require.Error(t, err)
				require.Equal(t, tc.expected.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
