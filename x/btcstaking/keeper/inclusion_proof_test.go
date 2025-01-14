package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func FuzzVerifyInclusionProofAndGetHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		// generate dummy staking tx data
		msgTx := datagen.CreateDummyTx()
		stakingTx := btcutil.NewTx(msgTx)
		confirmationDepth := uint32(6)
		stakingTime := uint32(1000)

		params := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate common merkle proof and inclusion header
		prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
		btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, msgTx)
		headerHash := btcHeaderWithProof.HeaderBytes.Hash()
		headerBytes, err := bbntypes.NewBTCHeaderBytesFromBytes(btcHeaderWithProof.HeaderBytes)
		require.NoError(t, err)
		inclusionHeight := uint32(datagen.RandomInt(r, 1000) + 1)
		inclusionHeader := &btclctypes.BTCHeaderInfo{
			Header: &headerBytes,
			Height: inclusionHeight,
		}

		// create inclusion proof
		proof := &types.ParsedProofOfInclusion{
			HeaderHash: headerHash,
			Proof:      btcHeaderWithProof.SpvProof.MerkleNodes,
			Index:      btcHeaderWithProof.SpvProof.BtcTransactionIndex,
		}

		// indicates the staking tx has room for unbonding
		maxValidTipHeight := inclusionHeight + stakingTime - params.UnbondingTimeBlocks - 1
		// indicates the staking tx is k-deep
		minValidTipHeight := inclusionHeight + confirmationDepth

		t.Run("successful verification", func(t *testing.T) {
			// Set the tip height to be in the range of valid min and max tip height
			tipHeight := datagen.RandomInt(r, int(maxValidTipHeight)-int(minValidTipHeight)+1) + uint64(minValidTipHeight)
			mockTipHeaderInfo := &btclctypes.BTCHeaderInfo{Height: uint32(tipHeight)}

			btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), headerHash).Return(inclusionHeader, nil).Times(1)
			btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTipHeaderInfo).Times(1)

			// Verify inclusion proof
			timeRange, err := h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
				h.Ctx,
				stakingTx,
				confirmationDepth,
				stakingTime,
				params.UnbondingTimeBlocks,
				proof,
			)

			require.NoError(t, err)
			require.Equal(t, inclusionHeader.Height, timeRange.StartHeight)
			require.Equal(t, inclusionHeader.Height+stakingTime, timeRange.EndHeight)
		})

		t.Run("nil inclusion header", func(t *testing.T) {
			// set the returned inclusion header as nil
			btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), headerHash).Return(nil, btclctypes.ErrHeaderDoesNotExist.Wrap("no header with provided hash")).Times(1)

			// Verify inclusion proof
			_, err := h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
				h.Ctx,
				stakingTx,
				confirmationDepth,
				stakingTime,
				params.UnbondingTimeBlocks,
				proof,
			)

			expErr := fmt.Errorf("staking tx inclusion proof header %s is not found in BTC light client state: %v", proof.HeaderHash.MarshalHex(), btclctypes.ErrHeaderDoesNotExist.Wrap("no header with provided hash"))
			require.EqualError(t, err, expErr.Error())
		})

		t.Run("invalid proof", func(t *testing.T) {
			btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), headerHash).Return(inclusionHeader, nil).Times(1)

			copyProof := *proof
			// make the proof invalid by setting the index to a different value
			copyProof.Index = proof.Index + 1
			_, err = h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
				h.Ctx,
				stakingTx,
				confirmationDepth,
				stakingTime,
				params.UnbondingTimeBlocks,
				&copyProof,
			)

			require.ErrorContains(t, err, "not included in the Bitcoin chain")
		})

		t.Run("insufficient confirmation depth", func(t *testing.T) {
			tipHeight := inclusionHeight + uint32(datagen.RandomInt(r, int(confirmationDepth)))
			mockTipHeaderInfo := &btclctypes.BTCHeaderInfo{Height: tipHeight}

			btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), headerHash).Return(inclusionHeader, nil).Times(1)
			btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTipHeaderInfo).Times(1)

			// Verify inclusion proof
			_, err = h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
				h.Ctx,
				stakingTx,
				confirmationDepth,
				stakingTime,
				params.UnbondingTimeBlocks,
				proof,
			)

			require.ErrorContains(t, err, "not k-deep")
		})

		t.Run("insufficient unbonding time", func(t *testing.T) {
			tipHeight := datagen.RandomInt(r, 1000) + uint64(maxValidTipHeight) + 1
			mockTipHeaderInfo := &btclctypes.BTCHeaderInfo{Height: uint32(tipHeight)}

			btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), headerHash).Return(inclusionHeader, nil).Times(1)
			btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTipHeaderInfo).Times(1)

			// Verify inclusion proof
			_, err = h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
				h.Ctx,
				stakingTx,
				confirmationDepth,
				stakingTime,
				params.UnbondingTimeBlocks,
				proof,
			)

			require.ErrorContains(t, err, "staking tx's timelock has no more than unbonding")
		})

		t.Run("invalid min unbonding time", func(t *testing.T) {
			// Set the tip height to be in the range of valid min and max tip height
			tipHeight := datagen.RandomInt(r, int(maxValidTipHeight)-int(minValidTipHeight)+1) + uint64(minValidTipHeight)
			mockTipHeaderInfo := &btclctypes.BTCHeaderInfo{Height: uint32(tipHeight)}

			btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), headerHash).Return(inclusionHeader, nil).Times(1)
			btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTipHeaderInfo).Times(1)

			// an invalid min_unbonding_time should be >= end_height - tip_height
			invalidMinUnbondingTime := uint32(datagen.RandomInt(r, 1000)) + inclusionHeight + stakingTime - uint32(tipHeight)

			_, err = h.BTCStakingKeeper.VerifyInclusionProofAndGetHeight(
				h.Ctx,
				stakingTx,
				confirmationDepth,
				stakingTime,
				invalidMinUnbondingTime,
				proof,
			)

			require.ErrorContains(t, err, "staking tx's timelock has no more than unbonding")
		})
	})
}
