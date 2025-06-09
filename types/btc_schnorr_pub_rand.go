package types

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"github.com/babylonlabs-io/babylon/v3/crypto/eots"
)

type SchnorrPubRand []byte

const SchnorrPubRandLen = 32

func NewSchnorrPubRand(data []byte) (*SchnorrPubRand, error) {
	var pr SchnorrPubRand
	err := pr.Unmarshal(data)
	return &pr, err
}

func NewSchnorrPubRandFromHex(prHex string) (*SchnorrPubRand, error) {
	prBytes, err := hex.DecodeString(prHex)
	if err != nil {
		return nil, err
	}
	return NewSchnorrPubRand(prBytes)
}

func NewSchnorrPubRandFromFieldVal(r *btcec.FieldVal) *SchnorrPubRand {
	prBytes := r.Bytes()
	pr := SchnorrPubRand(prBytes[:])
	return &pr
}

func NewPubRandFromPrivRand(sr *eots.PrivateRand) *SchnorrPubRand {
	sk := secp256k1.NewPrivateKey(sr)
	var j secp256k1.JacobianPoint
	sk.PubKey().AsJacobian(&j)
	return NewSchnorrPubRandFromFieldVal(&j.X)
}

// ToFieldValNormalized converts bytes into btcec.FieldVal and ensures
// it is normalized
func (pr SchnorrPubRand) ToFieldValNormalized() *btcec.FieldVal {
	var r btcec.FieldVal
	if r.SetByteSlice(pr) {
		r.Normalize()
	}
	return &r
}

func (pr SchnorrPubRand) Size() int {
	return len(pr.MustMarshal())
}

func (pr SchnorrPubRand) Marshal() ([]byte, error) {
	return pr, nil
}

func (pr SchnorrPubRand) MarshalHex() string {
	return hex.EncodeToString(pr)
}

func (pr SchnorrPubRand) MustMarshal() []byte {
	prBytes, err := pr.Marshal()
	if err != nil {
		panic(err)
	}
	return prBytes
}

func (pr SchnorrPubRand) MarshalTo(data []byte) (int, error) {
	bz, err := pr.Marshal()
	if err != nil {
		return 0, err
	}
	copy(data, bz)
	return len(data), nil
}

func (pr *SchnorrPubRand) Unmarshal(data []byte) error {
	if len(data) != SchnorrPubRandLen {
		return fmt.Errorf("invalid data length")
	}
	*pr = data
	return nil
}

func (pr *SchnorrPubRand) ToHexStr() string {
	prBytes := pr.MustMarshal()
	return hex.EncodeToString(prBytes)
}
