package epoching_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/x/epoching"
	"github.com/stretchr/testify/require"

	simapp "github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/x/epoching/types"
)

func TestExportGenesis(t *testing.T) {
	app := simapp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)

	if err := app.EpochingKeeper.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	genesisState := epoching.ExportGenesis(ctx, app.EpochingKeeper)
	require.Equal(t, genesisState.Params, types.DefaultParams())
}

func TestInitGenesis(t *testing.T) {
	app := simapp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)

	genesisState := types.GenesisState{
		Params: types.Params{
			EpochInterval: 100,
		},
	}

	epoching.InitGenesis(ctx, app.EpochingKeeper, genesisState)
	require.Equal(t, app.EpochingKeeper.GetParams(ctx).EpochInterval, uint64(100))
}
