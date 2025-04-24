package monitor_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v2/x/monitor"
	"github.com/stretchr/testify/require"

	simapp "github.com/babylonlabs-io/babylon/v2/app"
	"github.com/babylonlabs-io/babylon/v2/x/monitor/types"
)

func TestExportGenesis(t *testing.T) {
	app := simapp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)
	genesisState := monitor.ExportGenesis(ctx, app.MonitorKeeper)
	require.Equal(t, genesisState, types.DefaultGenesis())
}
