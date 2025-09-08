package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r    = rand.New(rand.NewSource(seed))
			ctrl = gomock.NewController(t)
		)
		defer ctrl.Finish()

		gs := datagen.GenRandomZoneconciergeGenState(r)

		// mock btcstkconsumer keeper
		btcStkConsumerKeeper := types.NewMockBTCStkConsumerKeeper(ctrl)
		btcStkConsumerKeeper.EXPECT().GetAllRegisteredCosmosConsumers(gomock.Any()).Return([]*btcstkconsumertypes.ConsumerRegister{}).AnyTimes()
		btcStkConsumerKeeper.EXPECT().IsCosmosConsumer(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

		k, ctx := keepertest.ZoneConciergeKeeper(t, nil, nil, nil, nil, nil, nil, btcStkConsumerKeeper, nil)

		// set values to state using InitGenesis
		err := k.InitGenesis(ctx, *gs)
		require.NoError(t, err)

		// export stored module state
		exported, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortData(gs)
		types.SortData(exported)

		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gs := datagen.GenRandomZoneconciergeGenState(r)

		// mock btcstkconsumer keeper
		btcStkConsumerKeeper := types.NewMockBTCStkConsumerKeeper(ctrl)
		btcStkConsumerKeeper.EXPECT().GetAllRegisteredCosmosConsumers(gomock.Any()).Return([]*btcstkconsumertypes.ConsumerRegister{}).AnyTimes()
		btcStkConsumerKeeper.EXPECT().IsCosmosConsumer(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

		k, ctx := keepertest.ZoneConciergeKeeper(t, nil, nil, nil, nil, nil, nil, btcStkConsumerKeeper, nil)

		// Run the InitGenesis
		err := k.InitGenesis(ctx, *gs)
		require.NoError(t, err)

		// get the current state
		exported, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortData(gs)
		types.SortData(exported)

		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}
