package checkpointing_test

import (
	"math/rand"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// FuzzAddBLSSigVoteExtension_MultipleVals tests adding BLS signatures via VoteExtension
// with multiple validators
func FuzzAddBLSSigVoteExtension_MultipleVals(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// generate the validator set with 10 validators as genesis
		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ek := helper.App.EpochingKeeper
		ck := helper.App.CheckpointingKeeper

		epoch := ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// go to block 11, ensure the checkpoint is finalized
		interval := ek.GetParams(helper.Ctx).EpochInterval
		for i := uint64(0); i < interval; i++ {
			_, err := helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}

		epoch = ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)

		ckpt, err := ck.GetRawCheckpoint(helper.Ctx, epoch.EpochNumber-1)
		require.NoError(t, err)
		require.Equal(t, types.Sealed, ckpt.Status)
	})
}

// FuzzAddBLSSigVoteExtension_InsufficientVotingPower tests adding BLS signatures
// with insufficient voting power
func FuzzAddBLSSigVoteExtension_InsufficientVotingPower(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// generate the validator set with 10 validators as genesis
		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ek := helper.App.EpochingKeeper

		epoch := ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// the number of validators is less than 2/3 if the total set
		numOfValidators := datagen.RandomInt(r, 5) + 1
		genesisValSet.Keys = genesisValSet.Keys[:numOfValidators]
		interval := ek.GetParams(helper.Ctx).EpochInterval
		for i := uint64(0); i < interval-1; i++ {
			_, err := helper.ApplyEmptyBlockWithValSet(r, genesisValSet)
			if i < interval-2 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		}
	})
}

// FuzzAddBLSSigVoteExtension_InvalidVoteExtensions tests adding BLS signatures
// with invalid BLS signatures
func FuzzAddBLSSigVoteExtension_InvalidVoteExtensions(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		helper := testhelper.NewHelper(t)
		ek := helper.App.EpochingKeeper

		epoch := ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		interval := ek.GetParams(helper.Ctx).EpochInterval
		for i := uint64(0); i < interval-1; i++ {
			_, err := helper.ApplyEmptyBlockWithInvalidVoteExtensions(r)
			if i < interval-2 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		}
	})
}

// FuzzAddBLSSigVoteExtension_SomeInvalidVoteExtensions tests resilience
// of ProcessProposal against invalid vote extensions
func FuzzAddBLSSigVoteExtension_SomeInvalidVoteExtensions(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// generate the validator set with 10 validators as genesis
		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ek := helper.App.EpochingKeeper
		ck := helper.App.CheckpointingKeeper

		epoch := ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// go to block 10, ensure the checkpoint is finalized
		interval := ek.GetParams(helper.Ctx).EpochInterval
		for i := uint64(0); i < interval-2; i++ {
			_, err := helper.ApplyEmptyBlockWithSomeInvalidVoteExtensions(r)
			require.NoError(t, err)
		}
		// height 11, i.e., 1st block of next epoch
		_, err = helper.ApplyEmptyBlockWithSomeInvalidVoteExtensions(r)
		require.NoError(t, err)

		epoch = ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)

		ckpt, err := ck.GetRawCheckpoint(helper.Ctx, epoch.EpochNumber-1)
		require.NoError(t, err)
		require.Equal(t, types.Sealed, ckpt.Status)
	})
}

// FuzzExtendVote_InvalidBlockHash tests the case where the
// block hash for signing is invalid in terms of format
func FuzzExtendVote_InvalidBlockHash(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// generate the validator set with 10 validators as genesis
		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ek := helper.App.EpochingKeeper

		epoch := ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// go to block 10, reaching epoch boundary
		interval := ek.GetParams(helper.Ctx).EpochInterval
		for i := uint64(0); i < interval-2; i++ {
			_, err := helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}

		req1 := &abci.RequestExtendVote{
			Hash:   datagen.GenRandomByteArray(r, datagen.RandomIntOtherThan(r, types.HashSize, 100)),
			Height: 10,
		}
		_, err = helper.App.ExtendVote(helper.Ctx, req1)
		require.Error(t, err)

		req2 := &abci.RequestExtendVote{
			Hash:   datagen.GenRandomByteArray(r, types.HashSize),
			Height: 10,
		}
		_, err = helper.App.ExtendVote(helper.Ctx, req2)
		require.NoError(t, err)
	})
}

// FuzzExtendVote_EmptyBLSPrivKey tests the case where the
// BLS private key of the private signer is missing
func FuzzExtendVote_EmptyBLSPrivKey(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// generate the validator set with 10 validators as genesis
		genesisValSet, ps, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)

		// set the BLS private key to be nil to trigger panic
		ps.WrappedPV.Key.BlsPrivKey = nil
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, ps)
		ek := helper.App.EpochingKeeper

		epoch := ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// go to block 10, reaching epoch boundary
		interval := ek.GetParams(helper.Ctx).EpochInterval
		for i := uint64(0); i < interval-2; i++ {
			_, err := helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}

		req := &abci.RequestExtendVote{
			Hash:   datagen.GenRandomByteArray(r, types.HashSize),
			Height: 10,
		}

		// error is expected due to nil BLS private key
		_, err = helper.App.ExtendVote(helper.Ctx, req)
		require.Error(t, err)
	})
}

// FuzzExtendVote_NotInValidatorSet tests the case where the
// private signer is not in the validator set
func FuzzExtendVote_NotInValidatorSet(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// generate the validator set with 10 validators as genesis
		genesisValSet, ps, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)

		// the private signer is not included in the validator set
		helper := testhelper.NewHelperWithValSetNoSigner(t, genesisValSet, ps)

		ek := helper.App.EpochingKeeper

		epoch := ek.GetEpoch(helper.Ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// go to block 10, reaching epoch boundary
		interval := ek.GetParams(helper.Ctx).EpochInterval
		for i := uint64(0); i < interval-2; i++ {
			_, err := helper.ApplyEmptyBlockWithSomeInvalidVoteExtensions(r)
			require.NoError(t, err)
		}

		req := &abci.RequestExtendVote{
			Hash:   datagen.GenRandomByteArray(r, types.HashSize),
			Height: 10,
		}

		// error is expected because the BLS signer in not
		// in the validator set
		_, err = helper.App.ExtendVote(helper.Ctx, req)
		require.Error(t, err)
	})
}
