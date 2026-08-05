package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
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
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)

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
		fKeeper.HandleRewarding(ctx, int64(targetHeight), uint64(10000))

		nextHeight := fKeeper.GetNextHeightToReward(ctx)
		require.Zero(t, nextHeight,
			"next height is not updated when no blocks finalized. Act: %d", nextHeight)

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
		fKeeper.HandleRewarding(ctx, int64(targetHeight), uint64(10000))

		nextHeight = fKeeper.GetNextHeightToReward(ctx)
		expectedNextHeight := activatedHeight + firstBatchFinalized
		require.Equal(t, expectedNextHeight, nextHeight,
			"next height should be after first batch of finalized blocks. Exp: %d, Act: %d", expectedNextHeight, nextHeight)

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
		fKeeper.HandleRewarding(ctx, int64(targetHeight), uint64(10000))

		// Verify final state
		finalNextHeight := fKeeper.GetNextHeightToReward(ctx)
		expectedFinalHeight := expectedNextHeight + secondBatchFinalized
		require.Equal(t, expectedFinalHeight, finalNextHeight,
			"next height should be after second batch of finalized blocks")
	})
}

func TestHandleRewardingWithGapsOfUnfinalizedBlocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().Unix()))

	// Setup keepers
	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)

	fpPK, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)
	fKeeper.SetVotingPower(ctx, fpPK.MustMarshal(), 1, 1)

	// starts rewarding at block 1
	fKeeper.SetNextHeightToReward(ctx, 1)

	fKeeper.SetBlock(ctx, &types.IndexedBlock{
		Height:    1,
		AppHash:   datagen.GenRandomByteArray(r, 32),
		Finalized: true,
	})

	fKeeper.SetBlock(ctx, &types.IndexedBlock{
		Height:    2,
		AppHash:   datagen.GenRandomByteArray(r, 32),
		Finalized: false,
	})

	// adds the latest finalized block
	fKeeper.SetBlock(ctx, &types.IndexedBlock{
		Height:    3,
		AppHash:   datagen.GenRandomByteArray(r, 32),
		Finalized: true,
	})
	dc := types.NewVotingPowerDistCache()
	dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
		BtcPk:          fpPK,
		TotalBondedSat: 1,
	})
	fKeeper.SetVotingPowerDistCache(ctx, 1, dc)
	fKeeper.SetVotingPowerDistCache(ctx, 3, dc)

	iKeeper.EXPECT().
		RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return().
		Times(2) // number of finalized blocks processed

	fKeeper.HandleRewarding(ctx, 3, uint64(10000))

	actNextBlockToBeRewarded := fKeeper.GetNextHeightToReward(ctx)
	require.Equal(t, uint64(4), actNextBlockToBeRewarded)
}

func FuzzHandleRewardingLimits(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup keepers
		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)

		// Activate BTC staking protocol at a random height
		activatedHeight := datagen.RandomInt(r, 10) + 1
		fpPK, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		fKeeper.SetVotingPower(ctx, fpPK.MustMarshal(), activatedHeight, 1)

		totalBlocks := uint64(100)
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

		nextHeight := fKeeper.GetNextHeightToReward(ctx)
		require.Zero(t, nextHeight,
			"next height is not updated when no blocks finalized. Act: %d", nextHeight)

		// Second phase: Finalize some blocks
		limit := uint64(10)
		firstBatchFinalized := uint64(50)
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
			Times(int(limit))

		// Second call to HandleRewarding
		fKeeper.HandleRewarding(ctx, int64(targetHeight), limit)

		nextHeight = fKeeper.GetNextHeightToReward(ctx)
		expectedNextHeight := activatedHeight + limit
		require.Equal(t, expectedNextHeight, nextHeight,
			"next height should be after first batch of finalized blocks. Exp: %d, Act: %d", expectedNextHeight, nextHeight)

		// Expect rewards for second batch of finalized blocks
		iKeeper.EXPECT().
			RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return().
			Times(int(limit))

		// Final call to HandleRewarding
		fKeeper.HandleRewarding(ctx, int64(targetHeight), limit)

		// Verify final state
		finalNextHeight := fKeeper.GetNextHeightToReward(ctx)
		expectedFinalHeight := expectedNextHeight + limit
		require.Equal(t, expectedFinalHeight, finalNextHeight,
			"next height should be after second batch of finalized blocks")
	})
}
