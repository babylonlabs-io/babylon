package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

func FuzzHandleRewarding(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup keepers
		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)

		// Activate BTC staking protocol at a random height
		activatedHeight := datagen.RandomInt(r, 10) + 1
		fpPK, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		fKeeper.SetVotingPower(ctx, fpPK.MustMarshal(), activatedHeight, 1)

		totalBlocks := uint64(10)
		targetHeight := activatedHeight + totalBlocks - 1

		// First phase: Index blocks with none finalized
		for i := activatedHeight; i <= targetHeight; i++ {
			fKeeper.SetBlock(ctx, &types.IndexedBlock{
				Height:    i,
				AppHash:   datagen.GenRandomByteArray(r, 32),
				Finalized: false,
			})

			// Set voting power distribution cache for each height
			dc := types.NewVotingPowerDistCache()
			dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
				BtcPk:          fpPK,
				TotalBondedSat: 1,
			})
			fKeeper.SetVotingPowerDistCache(ctx, i, dc)
		}

		// First call to HandleRewarding - expect no rewards
		ctx = datagen.WithCtxHeight(ctx, targetHeight)
		fKeeper.HandleRewarding(ctx, int64(targetHeight))

		nextHeight := fKeeper.GetNextHeightToReward(ctx)
		require.Equal(t, uint64(0), nextHeight,
			"next height is not updated when no blocks finalized")

		// Second phase: Finalize some blocks
		firstBatchFinalized := datagen.RandomInt(r, 5) + 1
		for i := activatedHeight; i < activatedHeight+firstBatchFinalized; i++ {
			block, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			block.Finalized = true
			fKeeper.SetBlock(ctx, block)
		}

		// Expect rewards for first batch of finalized blocks
		iKeeper.EXPECT().
			RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return().
			Times(int(firstBatchFinalized))

		// Second call to HandleRewarding
		fKeeper.HandleRewarding(ctx, int64(targetHeight))

		nextHeight = fKeeper.GetNextHeightToReward(ctx)
		expectedNextHeight := activatedHeight + firstBatchFinalized
		require.Equal(t, expectedNextHeight, nextHeight,
			"next height should be after first batch of finalized blocks")

		// Third phase: Finalize more blocks
		secondBatchFinalized := datagen.RandomInt(r, int(totalBlocks-firstBatchFinalized)) + 1
		for i := expectedNextHeight; i < expectedNextHeight+secondBatchFinalized; i++ {
			block, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			block.Finalized = true
			fKeeper.SetBlock(ctx, block)
		}

		// Expect rewards for second batch of finalized blocks
		iKeeper.EXPECT().
			RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return().
			Times(int(secondBatchFinalized))

		// Final call to HandleRewarding
		fKeeper.HandleRewarding(ctx, int64(targetHeight))

		// Verify final state
		finalNextHeight := fKeeper.GetNextHeightToReward(ctx)
		expectedFinalHeight := expectedNextHeight + secondBatchFinalized
		require.Equal(t, expectedFinalHeight, finalNextHeight,
			"next height should be after second batch of finalized blocks")
	})
}
