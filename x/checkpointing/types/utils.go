package types

import (
	"bytes"
	"encoding/hex"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/btctxformatter"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
)

func (m BlsSig) Hash() BlsSigHash {
	fields := [][]byte{
		sdk.Uint64ToBigEndian(m.EpochNum),
		m.BlockHash.MustMarshal(),
		m.BlsSig.MustMarshal(),
		[]byte(m.SignerAddress),
	}
	return hash(fields)
}

func (m RawCheckpoint) Hash() RawCkptHash {
	fields := [][]byte{
		sdk.Uint64ToBigEndian(m.EpochNum),
		m.BlockHash.MustMarshal(),
		m.BlsMultiSig.MustMarshal(),
		m.Bitmap,
	}
	return hash(fields)
}

func (m RawCheckpoint) HashStr() string {
	return m.Hash().String()
}

// SignedMsg is the message corresponding to the BLS sig in this raw checkpoint
// Its value should be (epoch_number || app_hash)
func (m RawCheckpoint) SignedMsg() []byte {
	return append(sdk.Uint64ToBigEndian(m.EpochNum), *m.BlockHash...)
}

func hash(fields [][]byte) []byte {
	var bz []byte
	for _, b := range fields {
		bz = append(bz, b...)
	}
	return tmhash.Sum(bz)
}

func (m BlsSigHash) Bytes() []byte {
	return m
}

func (m RawCkptHash) Bytes() []byte {
	return m
}

func (m RawCkptHash) Equals(h RawCkptHash) bool {
	return bytes.Equal(m.Bytes(), h.Bytes())
}

func (m RawCkptHash) String() string {
	return hex.EncodeToString(m)
}

func FromStringToCkptHash(s string) (RawCkptHash, error) {
	return hex.DecodeString(s)
}

func FromBTCCkptBytesToRawCkpt(btcCkptBytes []byte) (*RawCheckpoint, error) {
	btcCkpt, err := btctxformatter.DecodeRawCheckpoint(btctxformatter.CurrentVersion, btcCkptBytes)
	if err != nil {
		return nil, err
	}
	return FromBTCCkptToRawCkpt(btcCkpt)
}

func FromBTCCkptToRawCkpt(btcCkpt *btctxformatter.RawBtcCheckpoint) (*RawCheckpoint, error) {
	var blockHash BlockHash
	err := blockHash.Unmarshal(btcCkpt.BlockHash)
	if err != nil {
		return nil, err
	}
	var blsSig bls12381.Signature
	err = blsSig.Unmarshal(btcCkpt.BlsSig)
	if err != nil {
		return nil, err
	}
	rawCheckpoint := &RawCheckpoint{
		EpochNum:    btcCkpt.Epoch,
		BlockHash:   &blockHash,
		Bitmap:      btcCkpt.BitMap,
		BlsMultiSig: &blsSig,
	}

	return rawCheckpoint, nil
}

func FromRawCkptToBTCCkpt(rawCkpt *RawCheckpoint, address []byte) (*btctxformatter.RawBtcCheckpoint, error) {
	appHashBytes, err := rawCkpt.BlockHash.Marshal()
	if err != nil {
		return nil, err
	}
	blsSigBytes, err := rawCkpt.BlsMultiSig.Marshal()
	if err != nil {
		return nil, err
	}

	btcCkpt := &btctxformatter.RawBtcCheckpoint{
		Epoch:            rawCkpt.EpochNum,
		BlockHash:        appHashBytes,
		BitMap:           rawCkpt.Bitmap,
		SubmitterAddress: address,
		BlsSig:           blsSigBytes,
	}

	return btcCkpt, nil
}

func GetSignBytes(epoch uint64, hash []byte) []byte {
	return append(sdk.Uint64ToBigEndian(epoch), hash...)
}
