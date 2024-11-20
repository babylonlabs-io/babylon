package keeper_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	"github.com/babylonlabs-io/babylon/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

func setupMsgServer(t testing.TB) (*keeper.Keeper, types.MsgServer, context.Context) {
	fKeeper, ctx := keepertest.FinalityKeeper(t, nil, nil, nil)
	return fKeeper, keeper.NewMsgServerImpl(*fKeeper), ctx
}

func TestMsgServer(t *testing.T) {
	_, ms, ctx := setupMsgServer(t)
	require.NotNil(t, ms)
	require.NotNil(t, ctx)
}

func FuzzCommitPubRandList(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, nil, cKeeper)
		ms := keeper.NewMsgServerImpl(*fKeeper)
		committedEpochNum := datagen.GenRandomEpochNum(r)
		cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: committedEpochNum}).AnyTimes()

		// create a random finality provider
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
		fpBTCPKBytes := fpBTCPK.MustMarshal()

		// Case 1: fail if the finality provider is not registered
		bsKeeper.EXPECT().HasFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(false).Times(1)
		startHeight := datagen.RandomInt(r, 10)
		numPubRand := uint64(200)
		_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.Error(t, err)
		// register the finality provider
		bsKeeper.EXPECT().HasFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()

		// Case 2: commit a list of <minPubRand pubrand and it should fail
		startHeight = datagen.RandomInt(r, 10)
		numPubRand = datagen.RandomInt(r, int(fKeeper.GetParams(ctx).MinPubRand))
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.Error(t, err)

		// Case 3: when the finality provider commits pubrand list and it should succeed
		startHeight = datagen.RandomInt(r, 10)
		numPubRand = 100 + datagen.RandomInt(r, int(fKeeper.GetParams(ctx).MinPubRand))
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.NoError(t, err)
		// query last public randomness and assert
		lastPrCommit := fKeeper.GetLastPubRandCommit(ctx, fpBTCPK)
		require.NotNil(t, lastPrCommit)
		require.Equal(t, committedEpochNum, lastPrCommit.EpochNum)
		committedHeight := datagen.RandomInt(r, int(numPubRand)) + startHeight
		commitByHeight, err := fKeeper.GetPubRandCommitForHeight(ctx, fpBTCPK, committedHeight)
		require.NoError(t, err)
		require.Equal(t, committedEpochNum, commitByHeight.EpochNum)

		// Case 4: commit a pubrand list with overlap of the existing pubrand in KVStore and it should fail
		overlappedStartHeight := startHeight + numPubRand - 1 - datagen.RandomInt(r, 5)
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, overlappedStartHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.Error(t, err)

		// Case 5: commit a pubrand list that has no overlap with existing pubrand and it should succeed
		nonOverlappedStartHeight := startHeight + numPubRand + datagen.RandomInt(r, 5)
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, nonOverlappedStartHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.NoError(t, err)
	})
}

func FuzzAddFinalitySig(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)
		ms := keeper.NewMsgServerImpl(*fKeeper)

		// create and register a random finality provider
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, "")
		require.NoError(t, err)
		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
		fpBTCPKBytes := fpBTCPK.MustMarshal()
		require.NoError(t, err)
		bsKeeper.EXPECT().HasFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()

		// set committed epoch num
		committedEpochNum := datagen.GenRandomEpochNum(r) + 1
		cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: committedEpochNum}).AnyTimes()

		// commit some public randomness
		startHeight := uint64(0)
		numPubRand := uint64(200)
		randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msgCommitPubRandList)
		require.NoError(t, err)

		// generate a vote
		blockHeight := startHeight + uint64(1)
		blockAppHash := datagen.GenRandomByteArray(r, 32)
		signer := datagen.GenRandomAccount().Address
		msg, err := datagen.NewMsgAddFinalitySig(signer, btcSK, startHeight, blockHeight, randListInfo, blockAppHash)
		require.NoError(t, err)

		// Case 0: fail if the committed epoch is not finalized
		lastFinalizedEpoch := datagen.RandomInt(r, int(committedEpochNum))
		o1 := cKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(lastFinalizedEpoch).Times(1)
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.ErrorIs(t, err, types.ErrPubRandCommitNotBTCTimestamped)

		// set the committed epoch finalized for the rest of the cases
		lastFinalizedEpoch = datagen.GenRandomEpochNum(r) + committedEpochNum
		cKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(lastFinalizedEpoch).After(o1).AnyTimes()

		// Case 1: fail if the finality provider does not have voting power
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 0)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.Error(t, err)

		// mock voting power
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)

		// Case 2: fail if the finality provider has not committed public randomness at that height
		blockHeight2 := startHeight + numPubRand + 1
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		msg.BlockHeight = blockHeight2
		_, err = ms.AddFinalitySig(ctx, msg)
		require.Error(t, err)
		// reset block height
		msg.BlockHeight = blockHeight

		// Case 3: successful if the finality provider has voting power and has not casted this vote yet
		// index this block first
		ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight), AppHash: blockAppHash})
		fKeeper.IndexBlock(ctx)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		// add vote and it should work
		_, err = ms.AddFinalitySig(ctx, msg)
		require.NoError(t, err)
		// query this vote and assert
		sig, err := fKeeper.GetSig(ctx, blockHeight, fpBTCPK)
		require.NoError(t, err)
		require.Equal(t, msg.FinalitySig.MustMarshal(), sig.MustMarshal())

		// Case 4: In case of duplicate vote return success
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.Error(t, err)

		// Case 5: the finality provider is slashed if it votes for a fork
		blockAppHash2 := datagen.GenRandomByteArray(r, 32)
		msg2, err := datagen.NewMsgAddFinalitySig(signer, btcSK, startHeight, blockHeight, randListInfo, blockAppHash2)
		require.NoError(t, err)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		// mock slashing interface
		bsKeeper.EXPECT().SlashFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(nil).Times(1)
		bsKeeper.EXPECT().PropagateFPSlashingToConsumers(gomock.Any(), gomock.Eq(fpBTCPK)).Return(nil).Times(1)

		// NOTE: even though this finality provider is slashed, the msg should be successful
		// Otherwise the saved evidence will be rolled back
		_, err = ms.AddFinalitySig(ctx, msg2)
		require.NoError(t, err)
		// ensure the evidence has been stored
		evidence, err := fKeeper.GetEvidence(ctx, fpBTCPK, blockHeight)
		require.NoError(t, err)
		require.Equal(t, msg2.BlockHeight, evidence.BlockHeight)
		require.Equal(t, msg2.FpBtcPk.MustMarshal(), evidence.FpBtcPk.MustMarshal())
		require.Equal(t, msg2.BlockAppHash, evidence.ForkAppHash)
		require.Equal(t, msg2.FinalitySig.MustMarshal(), evidence.ForkFinalitySig.MustMarshal())
		// extract the SK and assert the extracted SK is correct
		btcSK2, err := evidence.ExtractBTCSK()
		require.NoError(t, err)
		// ensure btcSK and btcSK2 are same or inverse, AND correspond to the same PK
		// NOTE: it's possible that different SKs derive to the same PK
		// In this scenario, signature of any of these SKs can be verified with this PK
		// exclude the first byte here since it denotes the y axis of PubKey, which does
		// not affect verification
		require.True(t, btcSK.Key.Equals(&btcSK2.Key) || btcSK.Key.Negate().Equals(&btcSK2.Key))
		require.Equal(t, btcSK.PubKey().SerializeCompressed()[1:], btcSK2.PubKey().SerializeCompressed()[1:])

		// Case 6: slashed finality provider cannot vote
		fp.SlashedBabylonHeight = blockHeight
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.ErrorIs(t, err, bstypes.ErrFpAlreadySlashed)

		// Case 7: jailed finality provider cannot vote
		fp.Jailed = true
		fp.SlashedBabylonHeight = 0
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.ErrorIs(t, err, bstypes.ErrFpAlreadyJailed)
	})
}

func FuzzUnjailFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, nil, cKeeper)
		ms := keeper.NewMsgServerImpl(*fKeeper)

		// create and register a random finality provider
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, "")
		require.NoError(t, err)
		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
		fpBTCPKBytes := fpBTCPK.MustMarshal()
		require.NoError(t, err)

		// set fp to be jailed
		fp.Jailed = true
		jailedTime := time.Now()
		signingInfo := types.FinalityProviderSigningInfo{
			FpBtcPk: fpBTCPK,
		}
		err = fKeeper.FinalityProviderSigningTracker.Set(ctx, fpBTCPK.MustMarshal(), signingInfo)
		require.NoError(t, err)

		// case 1: the signer's address does not match fp's address
		signer := datagen.GenRandomAccount().Address
		msg := &types.MsgUnjailFinalityProvider{
			Signer:  signer,
			FpBtcPk: fpBTCPK,
		}
		ctx = ctx.WithHeaderInfo(header.Info{Time: jailedTime.Add(1 * time.Second)})
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), fpBTCPKBytes).Return(fp, nil).AnyTimes()
		_, err = ms.UnjailFinalityProvider(ctx, msg)
		require.Equal(t, fmt.Sprintf("the fp's address %s does not match the signer %s of the requestion",
			fp.Addr, msg.Signer), err.Error())

		// case 2: unjail the fp when the jailing period is zero
		msg.Signer = fp.Addr
		_, err = ms.UnjailFinalityProvider(ctx, msg)
		require.ErrorIs(t, err, bstypes.ErrFpNotJailed)

		// case 3: unjail the fp when the jailing period is not passed
		msg.Signer = fp.Addr
		signingInfo.JailedUntil = jailedTime
		err = fKeeper.FinalityProviderSigningTracker.Set(ctx, fpBTCPK.MustMarshal(), signingInfo)
		require.NoError(t, err)
		ctx = ctx.WithHeaderInfo(header.Info{Time: jailedTime.Truncate(1 * time.Second)})
		_, err = ms.UnjailFinalityProvider(ctx, msg)
		require.ErrorIs(t, err, types.ErrJailingPeriodNotPassed)

		// case 4: unjail the fp when the jailing period is passed
		ctx = ctx.WithHeaderInfo(header.Info{Time: jailedTime.Add(1 * time.Second)})
		bsKeeper.EXPECT().UnjailFinalityProvider(ctx, fpBTCPKBytes).Return(nil).AnyTimes()
		_, err = ms.UnjailFinalityProvider(ctx, msg)
		require.NoError(t, err)
	})
}

func TestVoteForConflictingHashShouldRetrieveEvidenceAndSlash(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)
	ms := keeper.NewMsgServerImpl(*fKeeper)
	// create and register a random finality provider
	btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, "")
	require.NoError(t, err)
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
	fpBTCPKBytes := fpBTCPK.MustMarshal()
	require.NoError(t, err)
	bsKeeper.EXPECT().HasFinalityProvider(gomock.Any(),
		gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()
	cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: 1}).AnyTimes()
	cKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(uint64(1)).AnyTimes()
	// commit some public randomness
	startHeight := uint64(0)
	numPubRand := uint64(200)
	randListInfo, msgCommitPubRandList, err :=
		datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight,
			numPubRand)
	require.NoError(t, err)
	_, err = ms.CommitPubRandList(ctx, msgCommitPubRandList)
	require.NoError(t, err)
	// set a block height of 1 and some random list
	blockHeight := startHeight + uint64(1)

	// generate two random hashes, one for the canonical block and
	// one for a fork block
	canonicalHash := datagen.GenRandomByteArray(r, 32)
	forkHash := datagen.GenRandomByteArray(r, 32)
	signer := datagen.GenRandomAccount().Address
	require.NoError(t, err)
	// (1) Set a canonical hash at height 1
	ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight), AppHash: canonicalHash})
	fKeeper.IndexBlock(ctx)
	// (2) Vote for a different block at height 1, this will make us have
	// some "evidence"
	ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight), AppHash: forkHash})
	msg1, err := datagen.NewMsgAddFinalitySig(signer, btcSK, startHeight, blockHeight, randListInfo, forkHash)
	require.NoError(t, err)
	fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(),
		gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
	_, err = ms.AddFinalitySig(ctx, msg1)
	require.NoError(t, err)
	// (3) Now vote for the canonical block at height 1. This should slash Finality provider
	msg, err := datagen.NewMsgAddFinalitySig(signer, btcSK, startHeight, blockHeight, randListInfo, canonicalHash)
	ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight), AppHash: canonicalHash})
	require.NoError(t, err)
	fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(),
		gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
	bsKeeper.EXPECT().SlashFinalityProvider(gomock.Any(),
		gomock.Eq(fpBTCPKBytes)).Return(nil).Times(1)
	bsKeeper.EXPECT().PropagateFPSlashingToConsumers(gomock.Any(),
		gomock.Eq(fpBTCPK)).Return(nil).Times(1)
	_, err = ms.AddFinalitySig(ctx, msg)
	require.NoError(t, err)
	sig, err := fKeeper.GetSig(ctx, blockHeight, fpBTCPK)
	require.NoError(t, err)
	require.Equal(t, msg.FinalitySig.MustMarshal(),
		sig.MustMarshal())
}

func TestDoNotPanicOnNilProof(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)
	ms := keeper.NewMsgServerImpl(*fKeeper)

	// create and register a random finality provider
	btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, "")
	require.NoError(t, err)
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
	fpBTCPKBytes := fpBTCPK.MustMarshal()
	require.NoError(t, err)
	bsKeeper.EXPECT().HasFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()

	// set committed epoch num
	committedEpochNum := datagen.GenRandomEpochNum(r) + 1
	cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: committedEpochNum}).AnyTimes()

	// commit some public randomness
	startHeight := uint64(0)
	numPubRand := uint64(200)
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight, numPubRand)
	require.NoError(t, err)
	_, err = ms.CommitPubRandList(ctx, msgCommitPubRandList)
	require.NoError(t, err)

	// generate a vote
	blockHeight := startHeight + uint64(1)
	blockAppHash := datagen.GenRandomByteArray(r, 32)
	signer := datagen.GenRandomAccount().Address
	msg, err := datagen.NewMsgAddFinalitySig(
		signer,
		btcSK,
		startHeight,
		blockHeight,
		randListInfo,
		blockAppHash,
	)
	require.NoError(t, err)

	// Not panic on empty proof
	msg.Proof = nil
	// Case 3: successful if the finality provider has voting power and has not casted this vote yet
	// index this block first
	ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight), AppHash: blockAppHash})
	fKeeper.IndexBlock(ctx)
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).AnyTimes()
	// mock voting power
	fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
	// set the committed epoch finalized for the rest of the cases
	lastFinalizedEpoch := datagen.GenRandomEpochNum(r) + committedEpochNum
	cKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(lastFinalizedEpoch).AnyTimes()

	// add vote and it should work
	_, err = ms.AddFinalitySig(ctx, msg)
	require.Error(t, err)
}

func TestVerifyActivationHeight(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	fKeeper, ctx := keepertest.FinalityKeeper(t, nil, nil, nil)
	ms := keeper.NewMsgServerImpl(*fKeeper)
	err := fKeeper.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)
	activationHeight := fKeeper.GetParams(ctx).FinalityActivationHeight

	// checks pub rand commit
	btcSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	startHeight := activationHeight - 1
	numPubRand := uint64(200)
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight, numPubRand)
	require.NoError(t, err)

	_, err = ms.CommitPubRandList(ctx, msgCommitPubRandList)
	require.EqualError(t, err, types.ErrFinalityNotActivated.Wrapf(
		"public rand commit start block height: %d is lower than activation height %d",
		startHeight, activationHeight,
	).Error())

	// check finality vote
	blockHeight := activationHeight - 1
	blockAppHash := datagen.GenRandomByteArray(r, 32)
	signer := datagen.GenRandomAccount().Address
	msgFinality, err := datagen.NewMsgAddFinalitySig(
		signer,
		btcSK,
		startHeight,
		blockHeight,
		randListInfo,
		blockAppHash,
	)
	require.NoError(t, err)

	_, err = ms.AddFinalitySig(ctx, msgFinality)
	require.EqualError(t, err, types.ErrFinalityNotActivated.Wrapf(
		"finality block height: %d is lower than activation height %d",
		blockHeight, activationHeight,
	).Error())
}
