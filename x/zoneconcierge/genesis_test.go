package zoneconcierge_test

import (
	"testing"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		PortId: types.PortID,
		Params: types.Params{IbcPacketTimeoutSeconds: 100},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.ZoneConciergeKeeper(t, nil, nil, nil, nil, nil, nil, nil)
	zoneconcierge.InitGenesis(ctx, *k, genesisState)
	got := zoneconcierge.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	require.Equal(t, genesisState.PortId, got.PortId)
	require.Equal(t, genesisState.Params, got.Params)
}
