package zoneconcierge_test

import (
	"testing"

	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/testutil/nullify"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		PortId: types.PortID,
		Params: types.Params{IbcPacketTimeoutSeconds: 100},
	}

	k, ctx := keepertest.ZoneConciergeKeeper(t, nil, nil, nil, nil)
	zoneconcierge.InitGenesis(ctx, *k, genesisState)
	got := zoneconcierge.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	require.Equal(t, genesisState.PortId, got.PortId)
	require.Equal(t, genesisState.Params, got.Params)
}
