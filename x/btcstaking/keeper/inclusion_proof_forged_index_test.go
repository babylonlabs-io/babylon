package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
)

// TestRejectForgedIndexProof tests that the fix for vulnerability #63323 works correctly
// in the staking context. The vulnerability allowed attackers to submit the same transaction
// with forged indices (real_index + k * 2^proof_depth) which could:
// 1. Bypass the coinbase check by claiming index 0 as index 16
// 2. Create duplicate submissions with different indices for state bloat
//
// This test verifies that forged indices are now properly rejected.
func TestRejectForgedIndexProof(t *testing.T) {
	r := rand.New(rand.NewSource(12345)) // Fixed seed for reproducibility
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	h.GenAndApplyParams(r)
	params := h.BTCStakingKeeper.GetParams(h.Ctx)

	// Create a block with multiple transactions to get a meaningful proof depth
	numTxs := 10
	transactions := make([]*wire.MsgTx, numTxs)
	for i := 0; i < numTxs; i++ {
		transactions[i] = datagen.CreateDummyTx()
	}

	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.GenRandomBtcdBlockWithTransactions(r, transactions, &prevBlock.Header)

	// Use transaction at index 3 as our staking transaction
	stakingTxIdx := 3
	stakingTx := btcutil.NewTx(transactions[stakingTxIdx])
	confirmationDepth := uint32(6)
	stakingTime := uint32(1000)

	inclusionHeader := btcHeaderWithProof.Block.Header
	headerBytes := bbntypes.NewBTCHeaderBytesFromBlockHeader(&inclusionHeader)
	inclusionHeight := uint32(100)
	inclusionHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &headerBytes,
		Height: inclusionHeight,
	}
	inclusionHeaderHash := inclusionHeader.BlockHash()
	inclusionHeaderHashBytes := bbntypes.NewBTCHeaderHashBytesFromChainhash(&inclusionHeaderHash)

	// Create valid proof with correct index
	validProof := &types.ParsedProofOfInclusion{
		HeaderHash: &inclusionHeaderHashBytes,
		Proof:      btcHeaderWithProof.Proofs[stakingTxIdx].MerkleNodes,
		Index:      uint32(stakingTxIdx),
	}

	// Calculate the proof depth and the index multiplier
	// Proof depth = number of intermediate nodes in the Merkle proof
	proofDepth := uint32(len(validProof.Proof) / 32)
	indexMultiplier := uint32(1 << proofDepth)

	t.Logf("Testing with %d transactions, proof depth = %d, index multiplier = %d",
		numTxs, proofDepth, indexMultiplier)

	t.Run("valid proof with correct index succeeds", func(t *testing.T) {
		tipHeight := inclusionHeight + confirmationDepth + 10
		mockTipHeaderInfo := &btclctypes.BTCHeaderInfo{Height: tipHeight}

		btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), &inclusionHeaderHashBytes).Return(inclusionHeaderInfo, nil).Times(1)
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTipHeaderInfo).Times(1)

		timeRange, err := h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
			h.Ctx,
			stakingTx,
			confirmationDepth,
			stakingTime,
			params.UnbondingTimeBlocks,
			validProof,
		)

		require.NoError(t, err)
		require.Equal(t, inclusionHeight, timeRange.StartHeight)
		require.Equal(t, inclusionHeight+stakingTime, timeRange.EndHeight)
	})

	t.Run("forged index proof is rejected - prevents state bloat attack", func(t *testing.T) {
		// Create forged proof with index = real_index + 2^proof_depth
		// Before the fix, this would pass Merkle verification but create a different SubmissionKey
		forgedProof := &types.ParsedProofOfInclusion{
			HeaderHash: &inclusionHeaderHashBytes,
			Proof:      validProof.Proof, // Same proof bytes
			Index:      uint32(stakingTxIdx) + indexMultiplier, // Forged index
		}

		t.Logf("Attempting to submit with forged index %d (real index: %d, multiplier: %d)",
			forgedProof.Index, stakingTxIdx, indexMultiplier)

		btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), &inclusionHeaderHashBytes).Return(inclusionHeaderInfo, nil).Times(1)

		// This should now fail at Merkle proof verification
		_, err := h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
			h.Ctx,
			stakingTx,
			confirmationDepth,
			stakingTime,
			params.UnbondingTimeBlocks,
			forgedProof,
		)

		require.Error(t, err)
		require.ErrorContains(t, err, "not included in the Bitcoin chain",
			"Forged index should be rejected at Merkle proof verification")
	})

	t.Run("multiple forged indices all rejected", func(t *testing.T) {
		// Test multiple forged indices: real_index + k * 2^depth for k = 1, 2, 3
		for k := uint32(1); k <= 3; k++ {
			forgedIndex := uint32(stakingTxIdx) + k*indexMultiplier
			forgedProof := &types.ParsedProofOfInclusion{
				HeaderHash: &inclusionHeaderHashBytes,
				Proof:      validProof.Proof,
				Index:      forgedIndex,
			}

			btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), &inclusionHeaderHashBytes).Return(inclusionHeaderInfo, nil).Times(1)

			_, err := h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
				h.Ctx,
				stakingTx,
				confirmationDepth,
				stakingTime,
				params.UnbondingTimeBlocks,
				forgedProof,
			)

			require.Error(t, err)
			require.ErrorContains(t, err, "not included in the Bitcoin chain",
				"Forged index %d (real: %d + %d * %d) should be rejected",
				forgedIndex, stakingTxIdx, k, indexMultiplier)

			t.Logf("✓ Forged index %d correctly rejected", forgedIndex)
		}
	})
}

// TestRejectForgedCoinbaseIndex specifically tests that the coinbase bypass attack
// (Scenario A from vulnerability #63323) is prevented. An attacker who is a Bitcoin
// miner could create a coinbase transaction in staking format and claim it's at a
// non-zero index to bypass the coinbase check. This test verifies the fix prevents this.
func TestRejectForgedCoinbaseIndex(t *testing.T) {
	r := rand.New(rand.NewSource(54321))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	h.GenAndApplyParams(r)
	params := h.BTCStakingKeeper.GetParams(h.Ctx)

	// Create a block with coinbase + multiple transactions
	numTxs := 8
	transactions := make([]*wire.MsgTx, numTxs)
	for i := 0; i < numTxs; i++ {
		transactions[i] = datagen.CreateDummyTx()
	}

	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.GenRandomBtcdBlockWithTransactions(r, transactions, &prevBlock.Header)

	coinbaseIdx := 0
	coinbaseTx := btcutil.NewTx(btcHeaderWithProof.Transactions[coinbaseIdx])
	confirmationDepth := uint32(6)
	stakingTime := uint32(1000)

	inclusionHeader := btcHeaderWithProof.Block.Header
	headerBytes := bbntypes.NewBTCHeaderBytesFromBlockHeader(&inclusionHeader)
	inclusionHeight := uint32(100)
	inclusionHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &headerBytes,
		Height: inclusionHeight,
	}
	inclusionHeaderHash := inclusionHeader.BlockHash()
	inclusionHeaderHashBytes := bbntypes.NewBTCHeaderHashBytesFromChainhash(&inclusionHeaderHash)

	coinbaseProof := btcHeaderWithProof.Proofs[coinbaseIdx].MerkleNodes
	proofDepth := uint32(len(coinbaseProof) / 32)
	indexMultiplier := uint32(1 << proofDepth)

	t.Logf("Coinbase transaction at index 0, proof depth = %d, multiplier = %d", proofDepth, indexMultiplier)

	t.Run("coinbase at real index 0 is rejected", func(t *testing.T) {
		realCoinbaseProof := &types.ParsedProofOfInclusion{
			HeaderHash: &inclusionHeaderHashBytes,
			Proof:      coinbaseProof,
			Index:      0, // Real coinbase index
		}

		btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), &inclusionHeaderHashBytes).Return(inclusionHeaderInfo, nil).Times(1)

		_, err := h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
			h.Ctx,
			coinbaseTx,
			confirmationDepth,
			stakingTime,
			params.UnbondingTimeBlocks,
			realCoinbaseProof,
		)

		require.Error(t, err)
		require.ErrorContains(t, err, "coinbase tx cannot be used for staking")
	})

	t.Run("coinbase with forged non-zero index is rejected at proof verification", func(t *testing.T) {
		// Before the fix: attacker claims coinbase is at index = 2^depth
		// This would pass Merkle verification AND bypass the coinbase check (index != 0)
		// After the fix: rejected at Merkle proof verification
		forgedCoinbaseProof := &types.ParsedProofOfInclusion{
			HeaderHash: &inclusionHeaderHashBytes,
			Proof:      coinbaseProof,
			Index:      indexMultiplier, // Forged index: 0 + 1 * 2^depth
		}

		t.Logf("Attempting coinbase bypass with forged index %d", forgedCoinbaseProof.Index)

		btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), &inclusionHeaderHashBytes).Return(inclusionHeaderInfo, nil).Times(1)

		_, err := h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
			h.Ctx,
			coinbaseTx,
			confirmationDepth,
			stakingTime,
			params.UnbondingTimeBlocks,
			forgedCoinbaseProof,
		)

		require.Error(t, err)
		require.ErrorContains(t, err, "not included in the Bitcoin chain",
			"Forged coinbase index should be rejected at Merkle proof verification, preventing coinbase bypass")

		t.Log("✓ Coinbase bypass attack prevented: forged index rejected before reaching coinbase check")
	})
}
