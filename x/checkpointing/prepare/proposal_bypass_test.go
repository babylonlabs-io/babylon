package prepare_test

import (
	"fmt"
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/prepare"
	ckpttypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

func setupProposalHandler(t *testing.T, tempApp *app.BabylonApp) *prepare.ProposalHandler {
	encCfg := appparams.DefaultEncodingConfig()
	ckpttypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	mCfg := mempool.DefaultPriorityNonceMempoolConfig()
	mCfg.MaxTx = 0
	mem := mempool.NewPriorityMempool(mCfg)
	logger := log.NewTestLogger(t)
	db := dbm.NewMemDB()
	bApp := baseapp.NewBaseApp(t.Name(), logger, db, encCfg.TxConfig.TxDecoder(), baseapp.SetChainID("test"))

	ckptKeeper := tempApp.CheckpointingKeeper
	return prepare.NewProposalHandler(log.NewNopLogger(), ckptKeeper, mem, bApp, encCfg)
}

func TestDuplicateField_Issue(t *testing.T) {
	const (
		numFaultyValidators = 3
		paddingSizePerVal   = 600 // 600 bytes padding to exceed 1KB limit
		honestVoteExtSize   = 200 // ~200 bytes for honest vote extension
	)

	validSigner := datagen.GenRandomAddress().String()
	validValidator := datagen.GenRandomValidatorAddress().String()
	blockHash := make([]byte, 32)
	blsSig := make([]byte, 48)

	tempApp := app.NewTmpBabylonApp()
	interfaceRegistry := tempApp.InterfaceRegistry()

	t.Log("Creating malicious vote extensions")

	var maliciousVoteExts [][]byte
	for i := 0; i < numFaultyValidators; i++ {
		maliciousBytes := buildMaliciousVoteExtension(
			paddingSizePerVal,
			validSigner,
			validValidator,
			blockHash,
			1,
			100,
			blsSig,
		)
		maliciousVoteExts = append(maliciousVoteExts, maliciousBytes)
	}

	proposalHandler := setupProposalHandler(t, tempApp)
	ctx := setupSdkCtx(100)

	for i, maliciousBytes := range maliciousVoteExts {
		var ve ckpttypes.VoteExtension
		err := unknownproto.RejectUnknownFieldsStrict(maliciousBytes, &ve, interfaceRegistry)
		require.NoError(t, err, "Faulty validator %d: RejectUnknownFieldsStrict should pass", i+1)

		err = ve.Unmarshal(maliciousBytes)
		require.NoError(t, err, "Faulty validator %d: Unmarshal should pass", i+1)
		require.Equal(t, validSigner, ve.Signer, "Signer should be valid (last value wins)")

		bzVoteExtAfterParse, err := ve.Marshal()
		require.NoError(t, err)

		err = ve.Validate()
		require.NoError(t, err, "Faulty validator %d: Validate() should pass", i+1)

		// epoch 1 matches the EpochNum encoded in the malicious VE; the
		// malformed-bytes check fires before the epoch check regardless.
		blsSig, err := proposalHandler.VerifyVoteExtension(ctx, 1, maliciousBytes, blockHash)
		require.Nil(t, blsSig)
		require.EqualError(t, err, ckpttypes.ErrVoteExt.Wrapf(
			"malformed vote extension (possible malicious bytes included): original size %d, size after marshal %d",
			len(maliciousBytes), len(bzVoteExtAfterParse)).Error(),
		)
	}
}

// TestVerifyVoteExtension_RejectsWrongEpoch covers the proposer-side epoch
// check. A validator constructs a properly signed vote extension over
// GetSignBytes(claimedEpoch, blockHash) for a claimedEpoch that differs
// from the epoch the proposer is building a checkpoint for.
// VerifyVoteExtension must reject it before the per-sig VerifyBLSSig step
// (which would otherwise verify the sig against the VE's own claimed
// epoch and pass).
func TestVerifyVoteExtension_RejectsWrongEpoch(t *testing.T) {
	const (
		expectedEpoch = uint64(7)
		claimedEpoch  = uint64(5)
	)

	tempApp := app.NewTmpBabylonApp()
	proposalHandler := setupProposalHandler(t, tempApp)
	ctx := setupSdkCtx(100)

	// One validator signs a VE over GetSignBytes(claimedEpoch, blockHash). The
	// signature itself is valid for that epoch — the only issue is that
	// claimedEpoch != expectedEpoch.
	val := genNTestValidators(t, 1)[0]
	bh := ckpttypes.BlockHash(make([]byte, ckpttypes.HashSize))
	ve := val.VoteExtension(&bh, claimedEpoch)
	veBytes, err := ve.Marshal()
	require.NoError(t, err)

	blsSig, err := proposalHandler.VerifyVoteExtension(ctx, expectedEpoch, veBytes, bh)
	require.Nil(t, blsSig)
	require.EqualError(t, err,
		fmt.Sprintf("vote extension epoch mismatch: expected %d, got %d", expectedEpoch, claimedEpoch))
}

func buildMaliciousVoteExtension(
	paddingSize int,
	validSigner string,
	validValidator string,
	blockHash []byte,
	epochNum uint64,
	height uint64,
	blsSig []byte,
) []byte {
	var buf []byte

	// Field 1 (Signer) - FIRST occurrence with garbage padding
	garbageData := make([]byte, paddingSize)
	for i := range garbageData {
		garbageData[i] = byte(i % 256)
	}
	buf = protowire.AppendTag(buf, 1, protowire.BytesType)
	buf = protowire.AppendString(buf, string(garbageData))

	// Field 1 (Signer) - SECOND occurrence with valid value
	buf = protowire.AppendTag(buf, 1, protowire.BytesType)
	buf = protowire.AppendString(buf, validSigner)

	// Field 2 (ValidatorAddress)
	buf = protowire.AppendTag(buf, 2, protowire.BytesType)
	buf = protowire.AppendString(buf, validValidator)

	// Field 3 (BlockHash) - custom type, encoded as raw bytes
	buf = protowire.AppendTag(buf, 3, protowire.BytesType)
	buf = protowire.AppendBytes(buf, blockHash)

	// Field 4 (EpochNum)
	buf = protowire.AppendTag(buf, 4, protowire.VarintType)
	buf = protowire.AppendVarint(buf, epochNum)

	// Field 5 (Height)
	buf = protowire.AppendTag(buf, 5, protowire.VarintType)
	buf = protowire.AppendVarint(buf, height)

	// Field 6 (BlsSig) - custom type, encoded as bytes
	buf = protowire.AppendTag(buf, 6, protowire.BytesType)
	buf = protowire.AppendBytes(buf, blsSig)

	return buf
}
