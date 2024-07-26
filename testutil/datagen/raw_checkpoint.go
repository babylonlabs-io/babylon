package datagen

import (
	"math/rand"

	"github.com/boljen/go-bitmap"

	txformat "github.com/babylonlabs-io/babylon/btctxformatter"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

const (
	numValidators = 100 // BTC timestamping only supports 100 validators
)

func GenFullBitmap() bitmap.Bitmap {
	bmBytes := make([]byte, txformat.BitMapLength)
	bm := bitmap.Bitmap(bmBytes)
	for i := 0; i < numValidators; i++ {
		bitmap.Set(bm, i, true)
	}
	return bm
}

// GenRandomBitmap generates a random bitmap for the validator set
// It returns a random bitmap and the number of validators in the subset
func GenRandomBitmap(r *rand.Rand) (bitmap.Bitmap, int) {
	bmBytes := GenRandomByteArray(r, txformat.BitMapLength)
	bm := bitmap.Bitmap(bmBytes)
	numSubset := 0
	for i := 0; i < numValidators; i++ {
		if bitmap.Get(bm, i) {
			numSubset++
		}
	}
	return bm, numSubset
}

func GetRandomRawBtcCheckpoint(r *rand.Rand) *txformat.RawBtcCheckpoint {
	rawCkpt := GenRandomRawCheckpoint(r)
	return &txformat.RawBtcCheckpoint{
		Epoch:            rawCkpt.EpochNum,
		BlockHash:        *rawCkpt.BlockHash,
		BitMap:           rawCkpt.Bitmap,
		SubmitterAddress: GenRandomByteArray(r, txformat.AddressLength),
		BlsSig:           rawCkpt.BlsMultiSig.Bytes(),
	}
}

func GenRandomRawCheckpointWithMeta(r *rand.Rand) *types.RawCheckpointWithMeta {
	ckptWithMeta := &types.RawCheckpointWithMeta{
		Ckpt:     GenRandomRawCheckpoint(r),
		Status:   GenRandomStatus(r),
		PowerSum: 0,
	}
	return ckptWithMeta
}

func GenRandomRawCheckpoint(r *rand.Rand) *types.RawCheckpoint {
	randomHashBytes := GenRandomBlockHash(r)
	randomBLSSig := GenRandomBlsMultiSig(r)
	return &types.RawCheckpoint{
		EpochNum:    GenRandomEpochNum(r),
		BlockHash:   &randomHashBytes,
		Bitmap:      bitmap.New(types.BitmapBits),
		BlsMultiSig: &randomBLSSig,
	}
}

// GenRandomSequenceRawCheckpointsWithMeta generates random checkpoints from epoch 0 to a random epoch
func GenRandomSequenceRawCheckpointsWithMeta(r *rand.Rand) []*types.RawCheckpointWithMeta {
	var topEpoch, finalEpoch uint64
	epoch1 := GenRandomEpochNum(r)
	epoch2 := GenRandomEpochNum(r)
	if epoch1 > epoch2 {
		topEpoch = epoch1
		finalEpoch = epoch2
	} else if epoch1 < epoch2 {
		topEpoch = epoch2
		finalEpoch = epoch1
	} else { // In the case they are equal, make the topEpoch one more
		topEpoch = epoch1 + 1
		finalEpoch = epoch2
	}
	var checkpoints []*types.RawCheckpointWithMeta
	for e := uint64(0); e <= topEpoch; e++ {
		ckpt := GenRandomRawCheckpointWithMeta(r)
		ckpt.Ckpt.EpochNum = e
		if e <= finalEpoch {
			ckpt.Status = types.Finalized
		}
		checkpoints = append(checkpoints, ckpt)
	}

	return checkpoints
}

func GenSequenceRawCheckpointsWithMeta(r *rand.Rand, tipEpoch uint64) []*types.RawCheckpointWithMeta {
	ckpts := make([]*types.RawCheckpointWithMeta, int(tipEpoch)+1)
	for e := uint64(0); e <= tipEpoch; e++ {
		ckpt := GenRandomRawCheckpointWithMeta(r)
		ckpt.Ckpt.EpochNum = e
		ckpts[int(e)] = ckpt
	}
	return ckpts
}

func GenerateBLSSigs(keys []bls12381.PrivateKey, msg []byte) []bls12381.Signature {
	var sigs []bls12381.Signature
	for _, privkey := range keys {
		sig := bls12381.Sign(privkey, msg)
		sigs = append(sigs, sig)
	}

	return sigs
}

func GenerateLegitimateRawCheckpoint(r *rand.Rand, privKeys []bls12381.PrivateKey) *types.RawCheckpoint {
	// number of validators, at least 4
	n := len(privKeys)
	// ensure sufficient signers
	signerNum := n*2/3 + 1
	epochNum := GenRandomEpochNum(r)
	blockHash := GenRandomBlockHash(r)
	msgBytes := types.GetSignBytes(epochNum, blockHash)
	sigs := GenerateBLSSigs(privKeys[:signerNum], msgBytes)
	multiSig, _ := bls12381.AggrSigList(sigs)
	bm := bitmap.New(types.BitmapBits)
	for i := 0; i < signerNum; i++ {
		bm.Set(i, true)
	}
	btcCheckpoint := &types.RawCheckpoint{
		EpochNum:    epochNum,
		BlockHash:   &blockHash,
		Bitmap:      bm,
		BlsMultiSig: &multiSig,
	}

	return btcCheckpoint
}

func GenRandomBlockHash(r *rand.Rand) types.BlockHash {
	return GenRandomByteArray(r, types.HashSize)
}

func GenRandomBlsMultiSig(r *rand.Rand) bls12381.Signature {
	return GenRandomByteArray(r, bls12381.SignatureSize)
}

// GenRandomStatus generates random status except for Finalized
func GenRandomStatus(r *rand.Rand) types.CheckpointStatus {
	return types.CheckpointStatus(r.Int31n(int32(len(types.CheckpointStatus_name) - 1)))
}
