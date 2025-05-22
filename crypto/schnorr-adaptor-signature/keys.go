package schnorr_adaptor_signature

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/v3/crypto/common"
	"github.com/btcsuite/btcd/btcec/v2"
)

// DecryptionKey is the decryption key in the adaptor
// signature scheme, noted by t in the paper
type DecryptionKey struct {
	btcec.ModNScalar
}

func NewDecryptionKeyFromModNScalar(scalar *btcec.ModNScalar) (*DecryptionKey, error) {
	if scalar.IsZero() {
		return nil, fmt.Errorf("the given scalar is zero")
	}

	// enforce using a scalar corresponding to an even encryption key
	ekPoint, err := common.ScalarBaseMultWithBlinding(scalar)
	if err != nil {
		return nil, err
	}
	ekPoint.ToAffine()
	if ekPoint.Y.IsOdd() {
		scalar = scalar.Negate()
	}

	return &DecryptionKey{*scalar}, nil
}

func NewDecryptionKeyFromBTCSK(btcSK *btcec.PrivateKey) (*DecryptionKey, error) {
	return NewDecryptionKeyFromModNScalar(&btcSK.Key)
}

func NewDecryptionKeyFromBytes(decKeyBytes []byte) (*DecryptionKey, error) {
	if len(decKeyBytes) != ModNScalarSize {
		return nil, fmt.Errorf(
			"the length of the given bytes for decryption key is incorrect (expected: %d, actual: %d)",
			ModNScalarSize,
			len(decKeyBytes),
		)
	}

	var decKeyScalar btcec.ModNScalar
	decKeyScalar.SetByteSlice(decKeyBytes)

	return NewDecryptionKeyFromModNScalar(&decKeyScalar)
}

func (dk *DecryptionKey) GetEncKey() (*EncryptionKey, error) {
	ekPoint, err := common.ScalarBaseMultWithBlinding(&dk.ModNScalar)
	if err != nil {
		return nil, err
	}
	// NOTE: we convert ekPoint to affine coordinates for consistency
	ekPoint.ToAffine()
	return &EncryptionKey{*ekPoint}, nil
}

func (dk *DecryptionKey) ToBTCSK() *btcec.PrivateKey {
	return &btcec.PrivateKey{Key: dk.ModNScalar}
}

func (dk *DecryptionKey) ToBytes() []byte {
	scalarBytes := dk.ModNScalar.Bytes()
	return scalarBytes[:]
}

type EncryptionKey struct {
	btcec.JacobianPoint
}

func NewEncryptionKeyFromJacobianPoint(point *btcec.JacobianPoint) (*EncryptionKey, error) {
	// ensure the point is not at infinity
	if (point.X.IsZero() && point.Y.IsZero()) || point.Z.IsZero() {
		return nil, fmt.Errorf("the given Jacobian point is at infinity")
	}

	// convert point to affine coordinates if necessary
	affinePoint := *point
	if !affinePoint.Z.IsOne() {
		affinePoint.ToAffine()
	}

	// enforce affinePoint to be an even point
	// this is needed since we cannot predict whether the given
	// point or public key is odd or even
	if affinePoint.Y.IsOdd() {
		affinePoint.Y.Negate(1).Normalize()
	}

	return &EncryptionKey{affinePoint}, nil
}

func NewEncryptionKeyFromBTCPK(btcPK *btcec.PublicKey) (*EncryptionKey, error) {
	var btcPKPoint btcec.JacobianPoint
	btcPK.AsJacobian(&btcPKPoint)
	return NewEncryptionKeyFromJacobianPoint(&btcPKPoint)
}

func NewEncryptionKeyFromBytes(encKeyBytes []byte) (*EncryptionKey, error) {
	if len(encKeyBytes) != JacobianPointSize {
		return nil, fmt.Errorf(
			"the length of the given bytes for encryption key is incorrect (expected: %d, actual: %d)",
			JacobianPointSize,
			len(encKeyBytes),
		)
	}
	btcPKPoint, err := btcec.ParseJacobian(encKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse encryption key bytes: %w", err)
	}
	return NewEncryptionKeyFromJacobianPoint(&btcPKPoint)
}

func (ek *EncryptionKey) ToBTCPK() (*btcec.PublicKey, error) {
	return btcec.ParsePubKey(ek.ToBytes())
}

func (ek *EncryptionKey) ToBytes() []byte {
	fieldValBytes := btcec.JacobianToByteSlice(ek.JacobianPoint)
	return fieldValBytes
}

func GenKeyPair() (*EncryptionKey, *DecryptionKey, error) {
	sk, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	dk, err := NewDecryptionKeyFromBTCSK(sk)
	if err != nil {
		return nil, nil, err
	}
	ek, err := dk.GetEncKey()
	if err != nil {
		return nil, nil, err
	}
	return ek, dk, nil
}
