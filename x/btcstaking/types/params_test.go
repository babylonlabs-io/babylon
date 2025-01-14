package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/stretchr/testify/require"
)

func TestHeightToVersionMap(t *testing.T) {
	testCases := []struct {
		name          string
		heightToVer   types.HeightToVersionMap
		height        uint64
		expectedVer   uint32
		expectedError bool
	}{
		{
			name:          "empty map returns error",
			heightToVer:   types.HeightToVersionMap{},
			height:        100,
			expectedError: true,
		},
		{
			name: "exact height match for first pair",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},

			height:      100,
			expectedVer: 1,
		},
		{
			name: "exact height match for second pair",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},

			height:      200,
			expectedVer: 2,
		},
		{
			name: "height between versions",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},
			height:      150,
			expectedVer: 1,
		},
		{
			name: "height after last version",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},
			height:      300,
			expectedVer: 2,
		},
		{
			name: "height before first version",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},
			height:        99,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			version, err := tc.heightToVer.GetVersionForHeight(tc.height)

			if tc.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedVer, version)
		})
	}
}
