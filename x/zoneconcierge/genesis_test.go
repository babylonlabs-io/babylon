package zoneconcierge_test

import (
	"testing"

	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.Params{IbcPacketTimeoutSeconds: 100},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock btcstkconsumer keeper
	btcStkConsumerKeeper := types.NewMockBTCStkConsumerKeeper(ctrl)
	btcStkConsumerKeeper.EXPECT().GetAllRegisteredConsumerIDs(gomock.Any()).Return([]string{}).AnyTimes()

	k, ctx := keepertest.ZoneConciergeKeeper(t, nil, nil, nil, nil, nil, nil, btcStkConsumerKeeper)
	zoneconcierge.InitGenesis(ctx, *k, genesisState)
	got := zoneconcierge.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	require.Equal(t, genesisState.Params, got.Params)
}
