package btctxformatter

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

type BabylonTag []byte

type FormatVersion uint8

type formatHeader struct {
	tag     BabylonTag
	version FormatVersion
	part    uint8
}

type BabylonData struct {
	Data  []byte
	Index uint8
}

type RawBtcCheckpoint struct {
	Epoch            uint64
	BlockHash        []byte
	BitMap           []byte
	SubmitterAddress []byte
	BlsSig           []byte
}

const (
	TagLength               = 4
	CurrentVersion          FormatVersion = 0
	FirstPartIndex          uint8         = 0
	SecondPartIndex         uint8         = 1
	HeaderLength            = TagLength + 1
	BlockHashLength         = 32
	BitMapLength            = 13
	AddressLength           = 20
	NumberOfParts           = 2
	FirstPartHashLength     = 10
	BlsSigLength            = 48
	EpochLength             = 8
	FirstPartLength         = HeaderLength + BlockHashLength + AddressLength + EpochLength + BitMapLength
	SecondPartLength        = HeaderLength + BlsSigLength + FirstPartHashLength
	RawBTCCheckpointLength  = EpochLength + BlockHashLength + BitMapLength + BlsSigLength + AddressLength
)

func getVerHalf(version FormatVersion, halfNumber uint8) uint8 {
	return (uint8(version) & 0xF) | (halfNumber << 4)
}

func encodeHeader(tag BabylonTag, version FormatVersion, halfNumber uint8) []byte {
	if len(tag) != TagLength {
		panic("Tag length mismatch: expected 4 bytes")
	}
	data := append([]byte(tag), getVerHalf(version, halfNumber))
	return data
}

func U64ToBEBytes(u uint64) []byte {
	u64bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(u64bytes, u)
	return u64bytes
}

func encodeFirstOpReturn(
	tag BabylonTag,
	version FormatVersion,
	epoch uint64,
	blockHash []byte,
	bitMap []byte,
	submitterAddress []byte,
) ([]byte, error) {
	if len(blockHash) != BlockHashLength || len(bitMap) != BitMapLength || len(submitterAddress) != AddressLength {
		return nil, errors.New("invalid input lengths")
	}

	var buffer bytes.Buffer
	buffer.Write(encodeHeader(tag, version, FirstPartIndex))
	buffer.Write(U64ToBEBytes(epoch))
	buffer.Write(blockHash)
	buffer.Write(bitMap)
	buffer.Write(submitterAddress)

	return buffer.Bytes(), nil
}

func getCheckSum(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:FirstPartHashLength]
}

func encodeSecondOpReturn(
	tag BabylonTag,
	version FormatVersion,
	firstOpReturnBytes []byte,
	blsSig []byte,
) ([]byte, error) {
	if len(blsSig) != BlsSigLength {
		return nil, errors.New("invalid BLS signature length")
	}

	var buffer bytes.Buffer
	buffer.Write(encodeHeader(tag, version, SecondPartIndex))
	buffer.Write(blsSig)
	buffer.Write(getCheckSum(firstOpReturnBytes[HeaderLength:]))

	return buffer.Bytes(), nil
}

func EncodeCheckpointData(
	tag BabylonTag,
	version FormatVersion,
	rawBTCCheckpoint *RawBtcCheckpoint,
) ([]byte, []byte, error) {
	if len(tag) != TagLength {
		return nil, nil, errors.New("tag should have 4 bytes")
	}

	firstHalf, err := encodeFirstOpReturn(
		tag,
		version,
		rawBTCCheckpoint.Epoch,
		raw
