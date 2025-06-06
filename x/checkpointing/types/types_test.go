package types_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

<<<<<<< HEAD
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
=======
	bls12381 "github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
>>>>>>> 91d5342 (chore(checkpointing): update validations (#1118))
)

// a single validator
func TestRawCheckpointWithMeta_Accumulate1(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	epochNum := uint64(2)
	n := 1
	totalPower := int64(10)
	ckptkeeper, ctx, _ := testkeeper.CheckpointingKeeper(t, nil, nil)
	blockHash := datagen.GenRandomBlockHash(r)
	msg := types.GetSignBytes(epochNum, blockHash)
	blsPubkeys, blsSigs := datagen.GenRandomPubkeysAndSigs(n, msg)
	ckpt, err := ckptkeeper.BuildRawCheckpoint(ctx, epochNum, blockHash)
	require.NoError(t, err)
	valSet := datagen.GenRandomValSet(n)
	err = ckpt.Accumulate(valSet, valSet[0].Addr, blsPubkeys[0], blsSigs[0], totalPower)
	require.NoError(t, err)
	require.Equal(t, types.Sealed, ckpt.Status)

	// accumulate the same BLS sig
	err = ckpt.Accumulate(valSet, valSet[0].Addr, blsPubkeys[0], blsSigs[0], totalPower)
	require.ErrorIs(t, err, types.ErrCkptNotAccumulating)
	require.Equal(t, types.Sealed, ckpt.Status)
}

// 4 validators
func TestRawCheckpointWithMeta_Accumulate4(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	epochNum := uint64(2)
	n := 4
	totalPower := int64(10) * int64(n)
	ckptkeeper, ctx, _ := testkeeper.CheckpointingKeeper(t, nil, nil)
	blockHash := datagen.GenRandomBlockHash(r)
	msg := types.GetSignBytes(epochNum, blockHash)
	blsPubkeys, blsSigs := datagen.GenRandomPubkeysAndSigs(n, msg)
	ckpt, err := ckptkeeper.BuildRawCheckpoint(ctx, epochNum, blockHash)
	require.NoError(t, err)
	valSet := datagen.GenRandomValSet(n)
	for i := 0; i < n; i++ {
		err = ckpt.Accumulate(valSet, valSet[i].Addr, blsPubkeys[i], blsSigs[i], totalPower)
		if i <= 1 {
			require.NoError(t, err)
			require.Equal(t, types.Accumulating, ckpt.Status)
		}
		if i == 2 {
			require.Equal(t, types.Sealed, ckpt.Status)
		}
		if i == 3 {
			require.ErrorIs(t, err, types.ErrCkptNotAccumulating)
			require.Equal(t, types.Sealed, ckpt.Status)
		}
	}
}

func TestRawCheckpoint_ValidateBasic(t *testing.T) {
	var (
		r                = rand.New(rand.NewSource(time.Now().Unix()))
		validBlockHash   = datagen.GenRandomBlockHash(r)
		validBlsMultiSig = datagen.GenRandomBlsMultiSig(r)
	)
	testCases := []struct {
		name      string
		ckpt      types.RawCheckpoint
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid checkpoint",
			ckpt: types.RawCheckpoint{
				EpochNum:    1,
				BlockHash:   &validBlockHash,
				Bitmap:      []byte{0x01},
				BlsMultiSig: &validBlsMultiSig,
			},
			expectErr: false,
		},
		{
			name: "nil bitmap",
			ckpt: types.RawCheckpoint{
				BlockHash:   &validBlockHash,
				Bitmap:      nil,
				BlsMultiSig: &validBlsMultiSig,
			},
			expectErr: true,
			errMsg:    "bitmap cannot be empty",
		},
		{
			name: "nil block hash",
			ckpt: types.RawCheckpoint{
				BlockHash:   nil,
				Bitmap:      []byte{0x01},
				BlsMultiSig: &validBlsMultiSig,
			},
			expectErr: true,
			errMsg:    "empty BlockHash",
		},
		{
			name: "invalid block hash length",
			ckpt: types.RawCheckpoint{
				BlockHash:   &types.BlockHash{0x01, 0x02}, // too short
				Bitmap:      []byte{0x01},
				BlsMultiSig: &validBlsMultiSig,
			},
			expectErr: true,
			errMsg:    "error validating block hash",
		},
		{
			name: "nil BLS signature",
			ckpt: types.RawCheckpoint{
				BlockHash:   &validBlockHash,
				Bitmap:      []byte{0x01},
				BlsMultiSig: nil,
			},
			expectErr: true,
			errMsg:    "empty BLSMultiSig",
		},
		{
			name: "invalid BLS signature length",
			ckpt: types.RawCheckpoint{
				BlockHash:   &validBlockHash,
				Bitmap:      []byte{0x01},
				BlsMultiSig: &bls12381.Signature{0x01, 0x02}, // too short
			},
			expectErr: true,
			errMsg:    "error validating BLS multi-signature",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ckpt.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
