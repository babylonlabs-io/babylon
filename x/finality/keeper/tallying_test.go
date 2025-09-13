package keeper_test

import (
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

func FuzzTallying_FinalizingNoBlock(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)

		// activate BTC staking protocol at a random height
		activatedHeight := datagen.RandomInt(r, 10) + 1

		// index a list of blocks, don't give them QCs, and tally them
		// Expect they are not finalised
		for i := activatedHeight; i < activatedHeight+10; i++ {
			// index blocks
			fKeeper.SetBlock(ctx, &types.IndexedBlock{
				Height:    i,
				AppHash:   datagen.GenRandomByteArray(r, 32),
				Finalized: false,
			})
			// this block does not have QC
			err := giveNoQCToHeight(r, ctx, fKeeper, i)
			require.NoError(t, err)
		}
		// mock activated height
		fKeeper.SetVotingPower(ctx, datagen.GenRandomByteArray(r, 32), activatedHeight, 1)
		// tally blocks and none of them should be finalised
		ctx = datagen.WithCtxHeight(ctx, activatedHeight+10-1)
		fKeeper.TallyBlocks(ctx, uint64(10000))
		for i := activatedHeight; i < activatedHeight+10; i++ {
			ib, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			require.False(t, ib.Finalized)
		}
	})
}

func FuzzTallying_FinalizingSomeBlocks(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)

		// activate BTC staking protocol at a random height
		activatedHeight := datagen.RandomInt(r, 10) + 1

		// index a list of blocks, give some of them QCs, and tally them.
		// Expect they are all finalised
		numWithQCs := datagen.RandomInt(r, 5) + 1
		for i := activatedHeight; i < activatedHeight+10; i++ {
			// index blocks
			fKeeper.SetBlock(ctx, &types.IndexedBlock{
				Height:    i,
				AppHash:   datagen.GenRandomByteArray(r, 32),
				Finalized: false,
			})
			if i < activatedHeight+numWithQCs {
				// this block has QC
				err := giveQCToHeight(r, ctx, fKeeper, i)
				require.NoError(t, err)
			} else {
				// this block does not have QC
				err := giveNoQCToHeight(r, ctx, fKeeper, i)
				require.NoError(t, err)
			}
		}
		// tally blocks and none of them should be finalised
		ctx = datagen.WithCtxHeight(ctx, activatedHeight+10-1)
		fKeeper.TallyBlocks(ctx, uint64(10000))
		for i := activatedHeight; i < activatedHeight+10; i++ {
			ib, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			if i < activatedHeight+numWithQCs {
				require.True(t, ib.Finalized)
			} else {
				require.False(t, ib.Finalized)
			}
		}
	})
}

func FuzzTallying_FinalizingAtMostMaxFinalizedBlocks(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)

		// activate BTC staking protocol at a random height
		activatedHeight := datagen.RandomInt(r, 10) + 1

		// index a list of blocks, give some of them QCs, and tally them.
		// Expect they are all finalised
		limit := uint64(datagen.RandomInRange(r, 10, 20))
		numWithQCs := uint64(datagen.RandomInRange(r, 50, 100))
		totalBlocks := uint64(datagen.RandomInRange(r, 100, 200))
		for i := activatedHeight; i < activatedHeight+totalBlocks; i++ {
			// index blocks
			fKeeper.SetBlock(ctx, &types.IndexedBlock{
				Height:    i,
				AppHash:   datagen.GenRandomByteArray(r, 32),
				Finalized: false,
			})
			if i < activatedHeight+numWithQCs {
				// this block has QC
				err := giveQCToHeight(r, ctx, fKeeper, i)
				require.NoError(t, err)
			} else {
				// this block does not have QC
				err := giveNoQCToHeight(r, ctx, fKeeper, i)
				require.NoError(t, err)
			}
		}

		for i := activatedHeight; i < activatedHeight+totalBlocks; i++ {
			ib, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			require.False(t, ib.Finalized)
		}

		// tally blocks and only blocks up to limit should be finalised
		ctx = datagen.WithCtxHeight(ctx, activatedHeight+totalBlocks-1)
		fKeeper.TallyBlocks(ctx, limit)
		for i := activatedHeight; i < activatedHeight+totalBlocks; i++ {
			ib, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			// only limit blocks should be finalised
			if i < activatedHeight+limit {
				require.True(t, ib.Finalized)
			} else {
				require.False(t, ib.Finalized)
			}
		}

		// next limit batch of blocks should be finalised
		ctx = datagen.WithCtxHeight(ctx, activatedHeight+totalBlocks-1)
		fKeeper.TallyBlocks(ctx, limit)
		for i := activatedHeight + limit; i < activatedHeight+totalBlocks; i++ {
			ib, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			// only limit blocks should be finalised
			if i < activatedHeight+2*limit {
				require.True(t, ib.Finalized)
			} else {
				require.False(t, ib.Finalized)
			}
		}
	})
}

func giveQCToHeight(r *rand.Rand, ctx sdk.Context, fKeeper *keeper.Keeper, height uint64) error {
	dc := types.NewVotingPowerDistCache()
	// 3 votes
	for i := 0; i < 3; i++ {
		votedFpPK, err := datagen.GenRandomBIP340PubKey(r)
		if err != nil {
			return err
		}
		fKeeper.SetVotingPower(ctx, votedFpPK.MustMarshal(), height, 1)
		dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
			BtcPk:          votedFpPK,
			TotalBondedSat: 1,
		})
		votedSig, err := bbn.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
		if err != nil {
			return err
		}
		fKeeper.SetSig(ctx, height, votedFpPK, votedSig)
	}
	// the rest of the finality providers do not vote
	fKeeper.SetVotingPower(ctx, datagen.GenRandomByteArray(r, 32), height, 1)
	fKeeper.SetVotingPowerDistCache(ctx, height, dc)
	return nil
}

func giveNoQCToHeight(r *rand.Rand, ctx sdk.Context, fKeeper *keeper.Keeper, height uint64) error {
	dc := types.NewVotingPowerDistCache()
	// 1 vote
	votedFpPK, err := datagen.GenRandomBIP340PubKey(r)
	if err != nil {
		return err
	}
	fKeeper.SetVotingPower(ctx, votedFpPK.MustMarshal(), height, 1)
	dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
		BtcPk:          votedFpPK,
		TotalBondedSat: 1,
	})
	votedSig, err := bbn.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
	if err != nil {
		return err
	}
	fKeeper.SetSig(ctx, height, votedFpPK, votedSig)

	// the other 3 finality providers
	for i := 0; i < 3; i++ {
		fpPK, err := datagen.GenRandomBIP340PubKey(r)
		if err != nil {
			return err
		}
		fKeeper.SetVotingPower(ctx, fpPK.MustMarshal(), height, 1)
		dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
			BtcPk:          fpPK,
			TotalBondedSat: 1,
		})
	}
	fKeeper.SetVotingPowerDistCache(ctx, height, dc)

	return nil
}

func FuzzConsecutiveFinalization(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)

		// activate BTC staking protocol at a random height
		activatedHeight := datagen.RandomInt(r, 10) + 1
		numBlockToInspect := uint64(30)
		// There will be a block in between activatedHeight and activatedHeight + numBlockToInspect
		// that woud not get necessary votes to be finalised
		firstNonFinalizedBlock := activatedHeight + 1 + datagen.RandomInt(r, 20)

		for i := activatedHeight; i < activatedHeight+numBlockToInspect; i++ {
			// index blocks
			fKeeper.SetBlock(ctx, &types.IndexedBlock{
				Height:    i,
				AppHash:   datagen.GenRandomByteArray(r, 32),
				Finalized: false,
			})

			if i == firstNonFinalizedBlock {
				// this block does not have QC
				err := giveNoQCToHeight(r, ctx, fKeeper, i)
				require.NoError(t, err)
			} else {
				// this block has QC
				err := giveQCToHeight(r, ctx, fKeeper, i)
				require.NoError(t, err)
			}
		}

		ctx = datagen.WithCtxHeight(ctx, activatedHeight+numBlockToInspect-1)
		fKeeper.TallyBlocks(ctx, uint64(10000))

		// all blocks up to firstNonFinalizedBlock must be finalised
		for i := activatedHeight; i < firstNonFinalizedBlock; i++ {
			ib, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			require.True(t, ib.Finalized)
		}

		// all blocks from the firstNonFinalizedBlock must not be finalised
		for i := firstNonFinalizedBlock; i < activatedHeight+numBlockToInspect; i++ {
			ib, err := fKeeper.GetBlock(ctx, i)
			require.NoError(t, err)
			require.False(t, ib.Finalized)
		}
	})
}
