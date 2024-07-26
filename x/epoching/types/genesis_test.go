package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/testutil/nullify"
	"github.com/babylonlabs-io/babylon/x/epoching"
	"github.com/babylonlabs-io/babylon/x/epoching/types"
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
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
