package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc:     "invalid genesis state, no btc header",
			genState: &types.GenesisState{},
			valid:    false,
		},
		{
			desc: "invalid genesis state",
			genState: &types.GenesisState{
				BtcHeaders: []*types.BTCHeaderInfo{&types.BTCHeaderInfo{
					Height: 1,
				}},
			},
			valid: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}
