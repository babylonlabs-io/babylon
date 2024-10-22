package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func FuzzBTCHeightIndex(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		keeper, ctx := keepertest.BTCStakingKeeper(t, btclcKeeper, nil, nil)

		// randomise Babylon height and BTC height
		babylonHeight := datagen.RandomInt(r, 100)
		ctx = datagen.WithCtxHeight(ctx, babylonHeight)
		btcHeight := uint32(datagen.RandomInt(r, 100))
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: btcHeight}).Times(1)
		keeper.IndexBTCHeight(ctx)

		// assert BTC height
		actualBtcHeight := keeper.GetBTCHeightAtBabylonHeight(ctx, babylonHeight)
		require.Equal(t, btcHeight, actualBtcHeight)
		// assert current BTC height
		curBtcHeight := keeper.GetCurrentBTCHeight(ctx)
		require.Equal(t, btcHeight, curBtcHeight)
	})
}
