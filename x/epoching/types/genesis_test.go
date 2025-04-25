package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v2/app"
	"github.com/babylonlabs-io/babylon/v2/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v2/x/epoching"
	"github.com/babylonlabs-io/babylon/v2/x/epoching/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	// This test requires setting up the staking module
	// Otherwise the epoching module cannot initialise the genesis validator set
	app := app.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)
	keeper := app.EpochingKeeper

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	epoching.InitGenesis(ctx, keeper, genesisState)
	got := epoching.ExportGenesis(ctx, keeper)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}

func TestGenesisState_Validate(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
		errMsg   string
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "valid genesis state",
			genState: &types.GenesisState{
				Params: types.Params{
					EpochInterval: 100,
				},
			},
			valid: true,
		},
		{
			desc:     "invalid genesis state - empty",
			genState: &types.GenesisState{},
			valid:    false,
			errMsg:   "epoch interval must be at least 2",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}
