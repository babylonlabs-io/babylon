package zoneconcierge_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/testutil/nullify"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.Params{IbcPacketTimeoutSeconds: 100},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock btcstkconsumer keeper
	btcStkConsumerKeeper := types.NewMockBTCStkConsumerKeeper(ctrl)
	btcStkConsumerKeeper.EXPECT().GetAllRegisteredCosmosConsumers(gomock.Any()).Return([]*btcstkconsumertypes.ConsumerRegister{}).AnyTimes()

	k, ctx := keepertest.ZoneConciergeKeeper(t, nil, nil, nil, nil, nil, nil,
		btcStkConsumerKeeper, nil)
	zoneconcierge.InitGenesis(ctx, *k, genesisState)
	got := zoneconcierge.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	require.Equal(t, genesisState.Params, got.Params)
}
