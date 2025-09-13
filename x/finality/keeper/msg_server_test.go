package keeper_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	testutil "github.com/babylonlabs-io/babylon/v4/testutil/incentives-helper"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/testutil/mocks"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
)

func setupMsgServer(t testing.TB) (*keeper.Keeper, types.MsgServer, context.Context) {
	fKeeper, ctx := keepertest.FinalityKeeper(t, nil, nil, nil, nil)
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
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, nil, cKeeper, nil)
		ms := keeper.NewMsgServerImpl(*fKeeper)
		committedEpochNum := datagen.GenRandomEpochNum(r)
		cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: committedEpochNum}).AnyTimes()

		// create a random finality provider
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
		fpBTCPKBytes := fpBTCPK.MustMarshal()

		signingContext := signingcontext.FpRandCommitContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

		// Case 1: fail if the finality provider is not registered
		bsKeeper.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(false).Times(1)
		startHeight := datagen.RandomInt(r, 10)
		numPubRand := uint64(200)
		_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, signingContext, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.Error(t, err)
		// register the finality provider
		bsKeeper.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()

		// Case 2: commit a list of <minPubRand pubrand and it should fail
		startHeight = datagen.RandomInt(r, 10)
		numPubRand = datagen.RandomInt(r, int(fKeeper.GetParams(ctx).MinPubRand))
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, signingContext, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.Error(t, err)

		// Case 3: when the finality provider commits pubrand list and it should succeed
		startHeight = datagen.RandomInt(r, 10)
		numPubRand = 100 + datagen.RandomInt(r, int(fKeeper.GetParams(ctx).MinPubRand))
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, signingContext, startHeight, numPubRand)
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
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, signingContext, overlappedStartHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.Error(t, err)

		// Case 5: commit a pubrand list that has no overlap with existing pubrand and it should succeed
		nonOverlappedStartHeight := startHeight + numPubRand + datagen.RandomInt(r, 5)
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, signingContext, nonOverlappedStartHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.NoError(t, err)

		// Case 6: commit a pubrand list that overflows when adding startHeight + numPubRand
		overflowStartHeight := math.MaxUint64 - datagen.RandomInt(r, 5)
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, signingContext, overflowStartHeight, numPubRand)
		require.NoError(t, err)
		err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
		require.ErrorContains(t, err, types.ErrOverflowInBlockHeight.Error())

		// Case 7: commit a pubrand list with startHeight too far into the future
		startHeight = 500_000
		_, msg, err = datagen.GenRandomMsgCommitPubRandList(r, btcSK, signingContext, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msg)
		require.Error(t, err)
		require.ErrorContains(t, err, fmt.Sprintf("start height %d is too far into the future", startHeight))
	})
}

func FuzzAddFinalitySig(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		bsKeeper.EXPECT().UpdateFinalityProvider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		bsKeeper.EXPECT().IsFinalityProviderDeleted(gomock.Any(), gomock.Any()).Return(false).Times(7)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)
		ms := keeper.NewMsgServerImpl(*fKeeper)

		commitRandContext := signingcontext.FpRandCommitContextV0(ctx.ChainID(), fKeeper.ModuleAddress())
		finalitySigContext := signingcontext.FpFinVoteContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

		// create and register a random finality provider
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, "", "")
		require.NoError(t, err)
		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
		fpBTCPKBytes := fpBTCPK.MustMarshal()
		require.NoError(t, err)
		bsKeeper.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()

		// set committed epoch num
		committedEpochNum := datagen.GenRandomEpochNum(r) + 1
		cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: committedEpochNum}).AnyTimes()

		// commit some public randomness
		startHeight := uint64(0)
		numPubRand := uint64(200)
		randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, commitRandContext, startHeight, numPubRand)
		require.NoError(t, err)
		_, err = ms.CommitPubRandList(ctx, msgCommitPubRandList)
		require.NoError(t, err)

		// generate a vote
		blockHeight := startHeight + uint64(1)
		blockAppHash := datagen.GenRandomByteArray(r, 32)
		signer := datagen.GenRandomAccount().Address
		msg, err := datagen.NewMsgAddFinalitySig(signer, btcSK, finalitySigContext, startHeight, blockHeight, randListInfo, blockAppHash)
		require.NoError(t, err)
		ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight)})
		fKeeper.IndexBlock(ctx)

		// Case 0: fail if the committed epoch is not finalized
		lastFinalizedEpoch := datagen.RandomInt(r, int(committedEpochNum))
		o1 := cKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(lastFinalizedEpoch).Times(2)
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.ErrorIs(t, err, types.ErrPubRandCommitNotBTCTimestamped)

		// set the committed epoch finalized for the rest of the cases
		lastFinalizedEpoch = datagen.GenRandomEpochNum(r) + committedEpochNum
		cKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(lastFinalizedEpoch).After(o1).AnyTimes()

		// Case 1: fail if the finality provider does not have voting power
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 0)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.Error(t, err)

		// mock voting power
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)

		// Case 2: fail if the finality provider has not committed public randomness at that height
		blockHeight2 := startHeight + numPubRand + 1
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
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
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.NoError(t, err)
		// query this vote and assert
		sig, err := fKeeper.GetSig(ctx, blockHeight, fpBTCPK)
		require.NoError(t, err)
		require.Equal(t, msg.FinalitySig.MustMarshal(), sig.MustMarshal())

		// Case 4: In case of duplicate vote return success
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.Error(t, err)

		// Case 5: the finality provider is slashed if it votes for a fork
		blockAppHash2 := datagen.GenRandomByteArray(r, 32)
		msg2, err := datagen.NewMsgAddFinalitySig(signer, btcSK, finalitySigContext, startHeight, blockHeight, randListInfo, blockAppHash2)
		require.NoError(t, err)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		// mock slashing interface
		bsKeeper.EXPECT().SlashFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(nil).Times(1)

		// NOTE: even though this finality provider is slashed, the msg should be successful
		// Otherwise the saved evidence will be rolled back
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
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
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.ErrorIs(t, err, bstypes.ErrFpAlreadySlashed)

		// Case 7: jailed finality provider cannot vote
		fp.Jailed = true
		fp.SlashedBabylonHeight = 0
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.ErrorIs(t, err, bstypes.ErrFpAlreadyJailed)

		// Case 8: vote rejected due to the block is finalized and timestamped
		fKeeper.SetBlock(ctx, &types.IndexedBlock{Height: blockHeight, Finalized: true})
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.ErrorIs(t, err, types.ErrSigHeightOutdated)

		// Case 9: add finality vote from a deleted finality provider
		msg.BlockHeight++
		ctx = ctx.WithHeaderInfo(header.Info{Height: int64(msg.BlockHeight)})
		fKeeper.IndexBlock(ctx)
		cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), msg.BlockHeight).Return(uint64(1)).Times(1)
		bsKeeper.EXPECT().IsFinalityProviderDeleted(gomock.Any(), msg.FpBtcPk).Return(true).Times(1)
		_, err = ms.AddFinalitySig(ctx, msg)
		require.EqualError(t, err, types.ErrFinalityProviderIsDeleted.Wrapf("fp_btc_pk_hex: %s", msg.FpBtcPk.MarshalHex()).Error())
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
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, nil, cKeeper, nil)
		ms := keeper.NewMsgServerImpl(*fKeeper)

		fpPopContext := signingcontext.FpPopContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

		// create and register a random finality provider
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, fpPopContext, "")
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
	bsKeeper.EXPECT().UpdateFinalityProvider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	bsKeeper.EXPECT().IsFinalityProviderDeleted(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)
	ms := keeper.NewMsgServerImpl(*fKeeper)
	// create and register a random finality provider
	btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	fpPopContext := signingcontext.FpPopContextV0(ctx.ChainID(), fKeeper.ModuleAddress())
	commitRandContext := signingcontext.FpRandCommitContextV0(ctx.ChainID(), fKeeper.ModuleAddress())
	finalitySigContext := signingcontext.FpFinVoteContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

	fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, fpPopContext, "")
	require.NoError(t, err)
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
	fpBTCPKBytes := fpBTCPK.MustMarshal()
	require.NoError(t, err)
	bsKeeper.EXPECT().BabylonFinalityProviderExists(gomock.Any(),
		gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()
	cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: 1}).AnyTimes()
	cKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(uint64(1)).AnyTimes()
	// commit some public randomness
	startHeight := uint64(0)
	numPubRand := uint64(200)
	randListInfo, msgCommitPubRandList, err :=
		datagen.GenRandomMsgCommitPubRandList(r, btcSK, commitRandContext, startHeight,
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
	msg1, err := datagen.NewMsgAddFinalitySig(signer, btcSK, finalitySigContext, startHeight, blockHeight, randListInfo, forkHash)
	require.NoError(t, err)
	fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
	cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), gomock.Any()).Return(uint64(1)).AnyTimes()
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(),
		gomock.Eq(fpBTCPKBytes)).Return(fp, nil).Times(1)
	_, err = ms.AddFinalitySig(ctx, msg1)
	require.NoError(t, err)
	// (3) Now vote for the canonical block at height 1. This should slash Finality provider
	msg, err := datagen.NewMsgAddFinalitySig(signer, btcSK, finalitySigContext, startHeight, blockHeight, randListInfo, canonicalHash)
	ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight), AppHash: canonicalHash})
	require.NoError(t, err)
	fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(),
		gomock.Any()).Return(fp, nil).Times(1)
	bsKeeper.EXPECT().SlashFinalityProvider(gomock.Any(),
		gomock.Eq(fpBTCPKBytes)).Return(nil).Times(1)
	_, err = ms.AddFinalitySig(ctx, msg)
	require.NoError(t, err)
	sig, err := fKeeper.GetSig(ctx, blockHeight, fpBTCPK)
	require.NoError(t, err)
	require.Equal(t, msg.FinalitySig.MustMarshal(),
		sig.MustMarshal())

	// ensure the evidence has been stored
	evidence, err := fKeeper.GetEvidence(ctx, fpBTCPK, blockHeight)
	require.NoError(t, err)
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
}

func TestDoNotPanicOnNilProof(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	bsKeeper.EXPECT().UpdateFinalityProvider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	bsKeeper.EXPECT().IsFinalityProviderDeleted(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)
	ms := keeper.NewMsgServerImpl(*fKeeper)

	// create and register a random finality provider
	btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	fpPopContext := signingcontext.FpPopContextV0(ctx.ChainID(), fKeeper.ModuleAddress())
	commitRandContext := signingcontext.FpRandCommitContextV0(ctx.ChainID(), fKeeper.ModuleAddress())
	finalitySigContext := signingcontext.FpPopContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

	fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, fpPopContext, "")
	require.NoError(t, err)
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
	fpBTCPKBytes := fpBTCPK.MustMarshal()
	require.NoError(t, err)
	bsKeeper.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(true).AnyTimes()

	// set committed epoch num
	committedEpochNum := datagen.GenRandomEpochNum(r) + 1
	cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: committedEpochNum}).AnyTimes()

	// commit some public randomness
	startHeight := uint64(0)
	numPubRand := uint64(200)
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, commitRandContext, startHeight, numPubRand)
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
		finalitySigContext,
		startHeight,
		blockHeight,
		randListInfo,
		blockAppHash,
	)
	require.NoError(t, err)

	// Not panic on empty proof (error on ValidateBasic)
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
	cKeeper.EXPECT().GetEpochByHeight(gomock.Any(), gomock.Any()).Return(lastFinalizedEpoch).AnyTimes()

	// fail on ValidateBasic before msg server
	err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
	require.ErrorContains(t, err, "empty inclusion proof")
}

func TestVerifyActivationHeight(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	fKeeper, ctx := keepertest.FinalityKeeper(t, nil, nil, nil, nil)
	ms := keeper.NewMsgServerImpl(*fKeeper)
	err := fKeeper.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)
	activationHeight := fKeeper.GetParams(ctx).FinalityActivationHeight

	// checks pub rand commit
	btcSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	startHeight := activationHeight - 1
	numPubRand := uint64(200)

	commitRandContext := signingcontext.FpRandCommitContextV0(ctx.ChainID(), fKeeper.ModuleAddress())
	finalitySigContext := signingcontext.FpPopContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, commitRandContext, startHeight, numPubRand)
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
		finalitySigContext,
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

func FuzzEquivocationEvidence(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)
		ms := keeper.NewMsgServerImpl(*fKeeper)

		// set params with activation height
		err := fKeeper.SetParams(ctx, types.DefaultParams())
		require.NoError(t, err)
		activationHeight := fKeeper.GetParams(ctx).FinalityActivationHeight

		// create and register a random finality provider
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		fpPopContext := signingcontext.FpPopContextV0(ctx.ChainID(), fKeeper.ModuleAddress())
		commitRandContext := signingcontext.FpRandCommitContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, btcSK, fpPopContext, "")
		require.NoError(t, err)
		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
		fpBTCPKBytes := fpBTCPK.MustMarshal()

		// test invalid case - without PubRand field in MsgEquivocationEvidence
		invalidMsg := &types.MsgEquivocationEvidence{
			Signer:                  datagen.GenRandomAccount().Address,
			FpBtcPkHex:              fpBTCPK.MarshalHex(),
			BlockHeight:             activationHeight,
			CanonicalAppHashHex:     hex.EncodeToString(datagen.GenRandomByteArray(r, 32)),
			ForkAppHashHex:          hex.EncodeToString(datagen.GenRandomByteArray(r, 32)),
			CanonicalFinalitySigHex: "",
			ForkFinalitySigHex:      "",
		}

		_, err = invalidMsg.ParseToEvidence()
		require.Error(t, err)

		// test valid case
		blockHeight := activationHeight + datagen.RandomInt(r, 100)

		// generate proper pub rand data
		startHeight := blockHeight
		numPubRand := uint64(200)
		randListInfo, _, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, commitRandContext, startHeight, numPubRand)
		require.NoError(t, err)

		// mock pub rand for evidence
		pubRand := &randListInfo.PRList[0]

		// mock canonical and fork app hash
		canonicalAppHash := datagen.GenRandomByteArray(r, 32)
		forkAppHash := datagen.GenRandomByteArray(r, 32)

		// generate proper EOTS signatures using the same private key and randomness
		// but different messages (canonical vs fork) - this is what allows secret key extraction
		// Use the private randomness that corresponds to the public randomness already generated
		sr := randListInfo.SRList[0]

		// Create canonical message (height || canonical app hash)
		canonicalMsg := append(sdk.Uint64ToBigEndian(blockHeight), canonicalAppHash...)
		canonicalSig, err := eots.Sign(btcSK, sr, canonicalMsg)
		require.NoError(t, err)

		// Create fork message (height || fork app hash) using SAME key and randomness
		forkMsg := append(sdk.Uint64ToBigEndian(blockHeight), forkAppHash...)
		forkSig, err := eots.Sign(btcSK, sr, forkMsg)
		require.NoError(t, err)

		msg := &types.MsgEquivocationEvidence{
			Signer:                  datagen.GenRandomAccount().Address,
			FpBtcPkHex:              fpBTCPK.MarshalHex(),
			BlockHeight:             blockHeight,
			PubRandHex:              pubRand.MarshalHex(),
			CanonicalAppHashHex:     hex.EncodeToString(canonicalAppHash),
			ForkAppHashHex:          hex.EncodeToString(forkAppHash),
			CanonicalFinalitySigHex: hex.EncodeToString(bbn.NewSchnorrEOTSSigFromModNScalar(canonicalSig).MustMarshal()),
			ForkFinalitySigHex:      hex.EncodeToString(bbn.NewSchnorrEOTSSigFromModNScalar(forkSig).MustMarshal()),
			SigningContext:          "", // TODO: test using actual context
		}

		// set block height in context to be >= evidence height
		blockAppHash := datagen.GenRandomByteArray(r, 32)
		ctx = ctx.WithHeaderInfo(header.Info{Height: int64(blockHeight), AppHash: blockAppHash})
		fKeeper.IndexBlock(ctx)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(fp, nil).AnyTimes()

		// mock voting power
		fKeeper.SetVotingPower(ctx, fpBTCPKBytes, blockHeight, 1)

		// mock slashing interface
		bsKeeper.EXPECT().SlashFinalityProvider(gomock.Any(), gomock.Eq(fpBTCPKBytes)).Return(nil)

		_, err = ms.EquivocationEvidence(ctx, msg)
		require.NoError(t, err)

		storedEvidence, err := fKeeper.GetEvidence(ctx, fpBTCPK, blockHeight)
		require.NoError(t, err)
		require.Equal(t, msg.FpBtcPkHex, storedEvidence.FpBtcPk.MarshalHex())
		require.Equal(t, msg.BlockHeight, storedEvidence.BlockHeight)
		require.Equal(t, msg.PubRandHex, storedEvidence.PubRand.MarshalHex())
		require.Equal(t, msg.CanonicalAppHashHex, hex.EncodeToString(storedEvidence.CanonicalAppHash))
		require.Equal(t, msg.ForkAppHashHex, hex.EncodeToString(storedEvidence.ForkAppHash))
		require.Equal(t, msg.CanonicalFinalitySigHex, hex.EncodeToString(storedEvidence.CanonicalFinalitySig.MustMarshal()))
		require.Equal(t, msg.ForkFinalitySigHex, hex.EncodeToString(storedEvidence.ForkFinalitySig.MustMarshal()))
	})
}

func TestBtcDelegationRewards(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	btcLightClientTipHeight := uint32(30)

	btclcKeeper := bstypes.NewMockBTCLightClientKeeper(ctrl)
	btccKForBtcStaking := bstypes.NewMockBtcCheckpointKeeper(ctrl)
	chKeeper := mocks.NewMockZoneConciergeChannelKeeper(ctrl)

	epochNumber := uint64(10)
	btccKForFinality := types.NewMockCheckpointingKeeper(ctrl)
	btccKForFinality.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: epochNumber}).AnyTimes()
	btccKForFinality.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(epochNumber).AnyTimes()

	h := testutil.NewIncentiveHelper(t, btclcKeeper, btccKForBtcStaking, btccKForFinality, chKeeper)
	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)
	h.SetFinalityActivationHeight(0)

	// generate and insert new finality provider
	stakingValueFp1Del1 := int64(2 * 10e8)
	stakingValueFp1Del2 := int64(4 * 10e8)
	stakingTime := uint16(1000)

	fp1SK, fp1PK, fp1 := h.CreateFinalityProvider(r)
	// commit some public randomness
	startHeight := uint64(0)
	h.FpAddPubRand(r, fp1SK, startHeight)

	_, _, fp1Del1, _ := h.CreateActiveBtcDelegation(r, covenantSKs, fp1PK, stakingValueFp1Del1, stakingTime, btcLightClientTipHeight)
	_, _, fp1Del2, _ := h.CreateActiveBtcDelegation(r, covenantSKs, fp1PK, stakingValueFp1Del2, stakingTime, btcLightClientTipHeight)

	// process the events of the activated BTC delegations
	h.BTCStakingKeeper.IndexBTCHeight(h.Ctx)
	h.FinalityKeeper.UpdatePowerDist(h.Ctx)
	h.IncentivesKeeper.ProcessRewardTrackerEventsAtHeight(h.Ctx, uint64(h.Ctx.HeaderInfo().Height))

	fp1CurrentRwd, err := h.IncentivesKeeper.GetFinalityProviderCurrentRewards(h.Ctx, fp1.Address())
	h.NoError(err)
	h.Equal(stakingValueFp1Del1+stakingValueFp1Del2, fp1CurrentRwd.TotalActiveSat.Int64())

	fp1Del1RwdTracker, err := h.IncentivesKeeper.GetBTCDelegationRewardsTracker(h.Ctx, fp1.Address(), fp1Del1.Address())
	h.NoError(err)
	h.Equal(stakingValueFp1Del1, fp1Del1RwdTracker.TotalActiveSat.Int64())

	fp1Del2RwdTracker, err := h.IncentivesKeeper.GetBTCDelegationRewardsTracker(h.Ctx, fp1.Address(), fp1Del2.Address())
	h.NoError(err)
	h.Equal(stakingValueFp1Del2, fp1Del2RwdTracker.TotalActiveSat.Int64())

	// fp1, del1 => 2_00000000
	// fp1, del2 => 4_00000000

	// if 1500ubbn are added as reward
	// del1 should receive 1/3 => 500
	// del2 should receive 2/3 => 1000
	rwdFp1 := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1500)))
	err = h.IncentivesKeeper.AddFinalityProviderRewardsForBtcDelegations(h.Ctx, fp1.Address(), rwdFp1)
	h.NoError(err)

	rwdFp1Del1 := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(500)))
	rwdFp1Del2 := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000)))

	fp1Del1Rwd, err := h.IncentivesKeeper.RewardGauges(h.Ctx, &ictvtypes.QueryRewardGaugesRequest{
		Address: fp1Del1.Address().String(),
	})
	h.NoError(err)
	h.Equal(fp1Del1Rwd.RewardGauges[ictvtypes.BTC_STAKER.String()].Coins.String(), rwdFp1Del1.String())

	fp1Del2Rwd, err := h.IncentivesKeeper.RewardGauges(h.Ctx, &ictvtypes.QueryRewardGaugesRequest{
		Address: fp1Del2.Address().String(),
	})
	h.NoError(err)
	h.Equal(fp1Del2Rwd.RewardGauges[ictvtypes.BTC_STAKER.String()].Coins.String(), rwdFp1Del2.String())
}

func TestBtcDelegationRewardsEarlyUnbondingAndExpire(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	btclcKeeper := bstypes.NewMockBTCLightClientKeeper(ctrl)
	btccKForBtcStaking := bstypes.NewMockBtcCheckpointKeeper(ctrl)
	chKeeper := mocks.NewMockZoneConciergeChannelKeeper(ctrl)

	epochNumber := uint64(10)
	btccKForFinality := types.NewMockCheckpointingKeeper(ctrl)
	btccKForFinality.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: epochNumber}).AnyTimes()
	btccKForFinality.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(epochNumber).AnyTimes()

	h := testutil.NewIncentiveHelper(t, btclcKeeper, btccKForBtcStaking, btccKForFinality, chKeeper)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)
	h.SetFinalityActivationHeight(0)

	// generate and insert new finality provider
	stakingValue := int64(2 * 10e8)
	stakingTime := uint16(1001)

	fpSK, fpPK, fp := h.CreateFinalityProvider(r)
	// commit some public randomness
	startHeight := uint64(0)
	h.FpAddPubRand(r, fpSK, startHeight)
	btcLightClientTipHeight := uint32(30)

	delSK, stakingTxHash, del, unbondingInfo := h.CreateActiveBtcDelegation(r, covenantSKs, fpPK, stakingValue, stakingTime, btcLightClientTipHeight)

	// process the events as active btc delegation
	h.BTCStakingKeeper.IndexBTCHeight(h.Ctx)
	h.FinalityKeeper.UpdatePowerDist(h.Ctx)
	h.IncentivesKeeper.ProcessRewardTrackerEventsAtHeight(h.Ctx, uint64(h.Ctx.HeaderInfo().Height))

	h.EqualBtcDelRwdTrackerActiveSat(fp.Address(), del.Address(), uint64(stakingValue))

	// Execute early unbonding
	btcLightClientTipHeight = uint32(45)
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: btcLightClientTipHeight}).AnyTimes()

	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	unbondingTx := del.MustGetUnbondingTx()
	stakingTx := del.MustGetStakingTx()

	serializedUnbondingTxWithWitness, _ := datagen.AddWitnessToUnbondingTx(
		t,
		stakingTx.TxOut[0],
		delSK,
		covenantSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		stakingTime,
		stakingValue,
		unbondingTx,
		h.Net,
	)

	h.BtcUndelegate(stakingTxHash, del, unbondingInfo, serializedUnbondingTxWithWitness, btcLightClientTipHeight)

	// increases one bbn block to get the voting power distribution cache
	// from the previous block
	h.CtxAddBlkHeight(1)
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: btcLightClientTipHeight}).AnyTimes()

	// process the events as early unbonding btc delegation
	h.BTCStakingKeeper.IndexBTCHeight(h.Ctx)
	h.FinalityKeeper.UpdatePowerDist(h.Ctx)
	h.IncentivesKeeper.ProcessRewardTrackerEventsAtHeight(h.Ctx, uint64(h.Ctx.HeaderInfo().Height))

	h.EqualBtcDelRwdTrackerActiveSat(fp.Address(), del.Address(), 0)

	// reaches the btc block of expire BTC delegation
	// an unbond event will be processed
	// but should not reduce the TotalActiveSat again
	h.CtxAddBlkHeight(1)

	btcLightClientTipHeight += uint32(stakingTime)
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: btcLightClientTipHeight}).AnyTimes()

	// process the events as expired btc delegation
	h.BTCStakingKeeper.IndexBTCHeight(h.Ctx)
	h.FinalityKeeper.UpdatePowerDist(h.Ctx)
	h.IncentivesKeeper.ProcessRewardTrackerEventsAtHeight(h.Ctx, uint64(h.Ctx.HeaderInfo().Height))

	h.EqualBtcDelRwdTrackerActiveSat(fp.Address(), del.Address(), 0)
}
