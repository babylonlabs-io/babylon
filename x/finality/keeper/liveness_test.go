package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

func FuzzHandleLiveness(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		bsKeeper.EXPECT().GetParams(gomock.Any()).Return(bstypes.Params{MaxActiveFinalityProviders: 100}).AnyTimes()
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)

		mockedHooks := types.NewMockFinalityHooks(ctrl)
		mockedHooks.EXPECT().AfterSluggishFinalityProviderDetected(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		fKeeper.SetHooks(mockedHooks)

		params := fKeeper.GetParams(ctx)
		fpPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), fpPk.MustMarshal()).Return(&bstypes.FinalityProvider{Sluggish: false}, nil).AnyTimes()
		signingInfo := types.NewFinalityProviderSigningInfo(
			fpPk,
			1,
			0,
		)
		err = fKeeper.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signingInfo)
		require.NoError(t, err)

		// activate BTC staking protocol at a random height
		activatedHeight := int64(datagen.RandomInt(r, 10) + 1)

		// for signed blocks, mark the finality provider as having signed
		height := activatedHeight
		for ; height < activatedHeight+params.SignedBlocksWindow; height++ {
			err := fKeeper.HandleFinalityProviderLiveness(ctx, fpPk, false, height)
			require.NoError(t, err)
		}
		signingInfo, err = fKeeper.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, int64(0), signingInfo.MissedBlocksCounter)

		minSignedPerWindow := params.MinSignedPerWindowInt()
		maxMissed := params.SignedBlocksWindow - minSignedPerWindow
		// for blocks up to the inactivity boundary, mark the finality provider as having not signed
		sluggishDetectedHeight := height + maxMissed + 1
		for ; height < sluggishDetectedHeight; height++ {
			err := fKeeper.HandleFinalityProviderLiveness(ctx, fpPk, true, height)
			require.NoError(t, err)
			signingInfo, err = fKeeper.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
			require.NoError(t, err)
			if height < sluggishDetectedHeight-1 {
				require.GreaterOrEqual(t, maxMissed, signingInfo.MissedBlocksCounter)
			} else {
				require.Less(t, maxMissed, signingInfo.MissedBlocksCounter)
			}
		}

		// perform heights that not missed, expect the sluggish is reverted
		bsKeeper.EXPECT().RevertSluggishFinalityProvider(gomock.Any(), fpPk.MustMarshal()).Return(nil).AnyTimes()
		sluggishRevertedHeight := height + maxMissed
		for ; height < sluggishRevertedHeight; height++ {
			err := fKeeper.HandleFinalityProviderLiveness(ctx, fpPk, false, height)
			require.NoError(t, err)
			signingInfo, err = fKeeper.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
			require.NoError(t, err)
			if height < sluggishRevertedHeight-1 {
				require.Less(t, maxMissed, signingInfo.MissedBlocksCounter)
			} else {
				// the sluggish fp is reverted
				require.Equal(t, maxMissed, signingInfo.MissedBlocksCounter)
			}
		}
	})
}
