package schnorr_adaptor_signature

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

// AdaptorSignature is the structure for a Schnorr adaptor signature
// as defined in the spec. It corresponds to the SchnorrFunEncryptedSignature
// format described in the spec.
//
// It is a triple (R, s', need_negation) where:
//   - `R` is the tweaked public randomness, which is derived from
//     offsetting public randomness R' sampled by the signer by
//     using encryption key T
//   - `sHat` is the secret s' in the adaptor signature
//   - `needNegation` is a bool value indicating whether decryption
//     key needs to be negated when decrypting a Schnorr signature
//     It is needed since (R, s') does not tell whether R'+T has odd
//     or even y index, thus does not tell whether decryption key needs
//     to be negated upon decryption.
type AdaptorSignature struct {
	r            btcec.JacobianPoint
	sHat         btcec.ModNScalar
	needNegation bool
}

// newAdaptorSignature creates a new AdaptorSignature with the given parameters.
// It copies the values to avoid unexpected modifications.
func newAdaptorSignature(r *btcec.JacobianPoint, sHat *btcec.ModNScalar, needNegation bool) *AdaptorSignature {
	var sig AdaptorSignature
	sig.r.Set(r)
	sig.sHat.Set(sHat)
	sig.needNegation = needNegation
	return &sig
}

// NewAdaptorSignatureFromBytes parses the given byte array to an adaptor signature.
// The format is:
// - r (33 bytes): The Jacobian point R
// - sHat (32 bytes): The scalar s'
// - needNegation (1 byte): Boolean flag indicating if decryption key needs negation
func NewAdaptorSignatureFromBytes(asigBytes []byte) (*AdaptorSignature, error) {
	if len(asigBytes) != AdaptorSignatureSize {
		return nil, fmt.Errorf(
			"the length of the given bytes for adaptor signature is incorrect (expected: %d, actual: %d)",
			AdaptorSignatureSize,
			len(asigBytes),
		)
	}

	// extract r
	r, err := btcec.ParseJacobian(asigBytes[0:JacobianPointSize])
	if err != nil {
		return nil, fmt.Errorf("failed to parse r: %w", err)
	}

	// extract sHat
	var sHat btcec.ModNScalar
	sHat.SetByteSlice(asigBytes[JacobianPointSize : JacobianPointSize+ModNScalarSize])

	// extract needNegation
	needNegation := asigBytes[JacobianPointSize+ModNScalarSize] == 0x01

	// Create a new AdaptorSignature
	// Since r is already a pointer, we can use newAdaptorSignature
	return newAdaptorSignature(&r, &sHat, needNegation), nil
}

// NewAdaptorSignatureFromHex parses the given hex string to an adaptor signature.
func NewAdaptorSignatureFromHex(asigHex string) (*AdaptorSignature, error) {
	asigBytes, err := hex.DecodeString(asigHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	return NewAdaptorSignatureFromBytes(asigBytes)
}

// Marshal serializes the adaptor signature to bytes.
// The format is:
// - r (33 bytes): The Jacobian point R
// - sHat (32 bytes): The scalar s'
// - needNegation (1 byte): Boolean flag indicating if decryption key needs negation
func (sig *AdaptorSignature) Marshal() ([]byte, error) {
	if sig == nil {
		return nil, nil
	}
	var asigBytes []byte
	// append r
	rBytes := btcec.JacobianToByteSlice(sig.r)
	asigBytes = append(asigBytes, rBytes...)
	// append sHat
	sHatBytes := sig.sHat.Bytes()
	asigBytes = append(asigBytes, sHatBytes[:]...)
	// append needNegation
	if sig.needNegation {
		asigBytes = append(asigBytes, 0x01)
	} else {
		asigBytes = append(asigBytes, 0x00)
	}
	return asigBytes, nil
}

// MustMarshal serializes the adaptor signature to bytes.
// It panics if marshaling fails.
func (sig *AdaptorSignature) MustMarshal() []byte {
	if sig == nil {
		return nil
	}
	bz, err := sig.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

// MarshalHex serializes the adaptor signature to a hex string.
func (sig *AdaptorSignature) MarshalHex() string {
	return hex.EncodeToString(sig.MustMarshal())
}

// Size returns the size of the serialized adaptor signature in bytes.
func (sig *AdaptorSignature) Size() int {
	return AdaptorSignatureSize
}

// MarshalTo serializes the adaptor signature to the provided byte slice.
func (sig *AdaptorSignature) MarshalTo(data []byte) (int, error) {
	bz, err := sig.Marshal()
	if err != nil {
		return 0, err
	}
	copy(data, bz)
	return len(data), nil
}

// Unmarshal deserializes an adaptor signature from bytes.
func (sig *AdaptorSignature) Unmarshal(data []byte) error {
	adaptorSig, err := NewAdaptorSignatureFromBytes(data)
	if err != nil {
		return err
	}

	*sig = *adaptorSig

	return nil
}

// Equals checks if two adaptor signatures are equal.
func (sig *AdaptorSignature) Equals(sig2 AdaptorSignature) bool {
	return bytes.Equal(sig.MustMarshal(), sig2.MustMarshal())
}

// ToPreSignature converts an adaptor signature to a pre-signature.
// The pre-signature is a 64-byte array containing:
// - The x-coordinate of R' (32 bytes)
// - s' (32 bytes)
//
// This implements the ConvertFromSchnorrFunEncryptedSignature algorithm from the spec.
func (sig *AdaptorSignature) ToPreSignature() []byte {
	// Step 1: Extract R and s' from adaptor signature
	R := sig.r
	sHat := sig.sHat

	// Step 2: Serialize R x-coordinate to 32 bytes
	var preSignature [64]byte
	R.X.PutBytesUnchecked(preSignature[0:32])

	// Step 3: Serialize s' to 32 bytes
	sHatBytes := sHat.Bytes()
	copy(preSignature[32:64], sHatBytes[:])

	return preSignature[:]
}

// NewAdaptorSignatureFromPreSignature creates a new adaptor signature from a pre-signature and encryption key.
// The pre-signature must be 64 bytes and the encryption key must be 32 bytes.
//
// This implements the ConvertToSchnorrFunEncryptedSignature algorithm from the spec.
func NewAdaptorSignatureFromPreSignature(psig []byte, encKey []byte) (*AdaptorSignature, error) {
	// Validate input lengths
	if len(psig) != 64 {
		return nil, fmt.Errorf("pre-signature must be 64 bytes, got %d", len(psig))
	}
	if len(encKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(encKey))
	}

	// Step 1: Let Rp = lift_x(int(psig[0:32]))
	var rX btcec.FieldVal
	if overflow := rX.SetByteSlice(psig[0:32]); overflow {
		return nil, fmt.Errorf("R x-coordinate exceeds field size")
	}
	Rp, err := liftX(&rX)
	if err != nil {
		return nil, fmt.Errorf("failed to lift R x-coordinate: %w", err)
	}

	// Step 2: Let s = int(psig[32:64])
	var s btcec.ModNScalar
	if overflow := s.SetByteSlice(psig[32:64]); overflow {
		return nil, fmt.Errorf("s value exceeds curve order")
	}

	// Step 3: Let Tp = lift_x(int(ek))
	var tX btcec.FieldVal
	if overflow := tX.SetByteSlice(encKey); overflow {
		return nil, fmt.Errorf("encryption key exceeds field size")
	}
	Tp, err := liftX(&tX)
	if err != nil {
		return nil, fmt.Errorf("failed to lift encryption key: %w", err)
	}

	// Step 4: Compute nn and RTp
	var RTp btcec.JacobianPoint
	var needNegation bool

	// Try Rp + Tp first
	btcec.AddNonConst(Rp, Tp, &RTp)
	RTp.ToAffine()

	if RTp.Y.IsOdd() {
		// If Rp+Tp has odd y, try Rp-Tp
		btcec.AddNonConst(Rp, negatePoint(Tp), &RTp)
		RTp.ToAffine()

		if RTp.Y.IsOdd() {
			return nil, fmt.Errorf("both Rp+Tp and Rp-Tp have odd y")
		}

		needNegation = true
	}

	// Step 5: Return (RTp, s, nn)
	return &AdaptorSignature{
		r:            *Rp,
		sHat:         s,
		needNegation: needNegation,
	}, nil
}
