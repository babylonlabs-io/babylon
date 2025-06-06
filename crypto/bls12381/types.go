package bls12381

import (
	"encoding/hex"
	"errors"
	"fmt"

	blst "github.com/supranational/blst/bindings/go"
)

// For minimal-pubkey-size operations:
// type BlsPubKey = blst.P1Affine
// type BlsSig = blst.P2Affine
// type BlsMultiSig = blst.P2Aggregate
// type BlsMultiPubKey = blst.P1Aggregate

// Domain Separation Tag for signatures on G2 (minimal-pubkey-size)
// const DST = []byte("BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_")

// For minimal-signature-size operations:
type BlsPubKey = blst.P2Affine
type BlsSig = blst.P1Affine
type BlsMultiSig = blst.P1Aggregate
type BlsMultiPubKey = blst.P2Aggregate

// Domain Separation Tag for signatures on G1 (minimal-signature-size)
var DST = []byte("BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_")

// Domain Separation Tag specified for the PoP ciphersuite
var DST_POP = []byte("BLS_POP_BLS12381G1_XMD:SHA-256_SSWU_RO_POP_")

type Signature []byte
type PublicKey []byte
type PrivateKey []byte

const (
	// SignatureSize is the size, in bytes, of a compressed BLS signature
	SignatureSize = 48
	// PrivKeySize is the size, in bytes, of a BLS private key
	PrivKeySize = 32
	// PubKeySize is the size, in bytes, of a compressed BLS public key
	PubKeySize = 96
	// SeedSize is the size, in bytes, of private key seeds
	SeedSize = 32
)

func (sig Signature) ValidateBasic() error {
	if sig == nil {
		return errors.New("empty BLS signature")
	}
	if len(sig) != SignatureSize {
		return fmt.Errorf("invalid BLS signature length, got %d, expected %d", len(sig), SignatureSize)
	}

	return nil
}

func (sig Signature) Marshal() ([]byte, error) {
	return sig, nil
}

func (sig Signature) MustMarshal() []byte {
	bz, err := sig.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

func (sig Signature) MarshalTo(data []byte) (int, error) {
	copy(data, sig)
	return len(data), nil
}

func (sig Signature) Size() int {
	bz, _ := sig.Marshal()
	return len(bz)
}

func (sig *Signature) Unmarshal(data []byte) error {
	if len(data) != SignatureSize {
		return fmt.Errorf("invalid BLS signature length, got %d, expected %d", len(data), SignatureSize)
	}

	*sig = data
	return nil
}

func (sig Signature) Bytes() []byte {
	return sig
}

func (sig Signature) Equal(s Signature) bool {
	return string(sig) == string(s)
}

func NewBLSSigFromHex(s string) (Signature, error) {
	bz, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	var sig Signature
	err = sig.Unmarshal(bz)
	if err != nil {
		return nil, err
	}

	return sig, nil
}

func (sig Signature) String() string {
	bz := sig.MustMarshal()

	return hex.EncodeToString(bz)
}

func (pk PublicKey) Marshal() ([]byte, error) {
	return pk, nil
}

func (pk PublicKey) MustMarshal() []byte {
	bz, err := pk.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

func (pk PublicKey) MarshalTo(data []byte) (int, error) {
	copy(data, pk)
	return len(data), nil
}

func (pk PublicKey) Size() int {
	bz, _ := pk.Marshal()
	return len(bz)
}

func (pk *PublicKey) Unmarshal(data []byte) error {
	if len(data) != PubKeySize {
		return fmt.Errorf("invalid BLS public key length, got %d, expected %d", len(data), PubKeySize)
	}

	*pk = data
	return nil
}

func (pk *PublicKey) ValidateBasic() error {
	if len(*pk) != PubKeySize {
		return fmt.Errorf("invalid BLS public key length, got %d, expected %d", len(*pk), PubKeySize)
	}

	// check the public key is a valid point on the BLS12-318 curve
	p2Affine := new(blst.P2Affine).Uncompress(*pk)

	if !p2Affine.KeyValidate() {
		return fmt.Errorf("invalid BLS public key point on the bls12-381 curve")
	}

	return nil
}

func (pk PublicKey) Equal(k PublicKey) bool {
	return string(pk) == string(k)
}

func (pk PublicKey) Bytes() []byte {
	return pk
}

func (sk PrivateKey) ValidateBasic() error {
	if len(sk) != PrivKeySize {
		return fmt.Errorf("invalid BLS private key length, got %d, expected %d", len(sk), PrivKeySize)
	}

	return nil
}

func (sk PrivateKey) PubKey() PublicKey {
	secretKey := new(blst.SecretKey)
	secretKey.Deserialize(sk)
	pk := new(BlsPubKey).From(secretKey)
	return pk.Compress()
}
