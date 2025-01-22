// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

package babylonclient

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCosmosProvider_AdjustEstimatedGas(t *testing.T) {
	testCases := []struct {
		name          string
		gasUsed       uint64
		gasAdjustment float64
		maxGasAmount  uint64
		expectedGas   uint64
		expectedErr   error
	}{
		{
			name:          "gas used is zero",
			gasUsed:       0,
			gasAdjustment: 1.0,
			maxGasAmount:  0,
			expectedGas:   0,
			expectedErr:   nil,
		},
		{
			name:          "gas used is non-zero",
			gasUsed:       50000,
			gasAdjustment: 1.5,
			maxGasAmount:  100000,
			expectedGas:   75000,
			expectedErr:   nil,
		},
		{
			name:          "gas used is infinite",
			gasUsed:       10000,
			gasAdjustment: math.Inf(1),
			maxGasAmount:  0,
			expectedGas:   0,
			expectedErr:   fmt.Errorf("infinite gas used"),
		},
		{
			name:          "gas used is non-zero with zero max gas amount as default",
			gasUsed:       50000,
			gasAdjustment: 1.5,
			maxGasAmount:  0,
			expectedGas:   75000,
			expectedErr:   nil,
		},
		{
			name:          "estimated gas is higher than max gas",
			gasUsed:       50000,
			gasAdjustment: 1.5,
			maxGasAmount:  70000,
			expectedGas:   75000,
			expectedErr:   fmt.Errorf("estimated gas 75000 is higher than max gas 70000"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cc := &CosmosProvider{PCfg: CosmosProviderConfig{
				GasAdjustment: tc.gasAdjustment,
				MaxGasAmount:  tc.maxGasAmount,
			}}
			adjustedGas, err := cc.AdjustEstimatedGas(tc.gasUsed)
			if err != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, adjustedGas, tc.expectedGas)
			}
		})
	}
}
