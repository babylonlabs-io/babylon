package schnorr_adaptor_signature

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

const (
	ModNScalarSize       = 32
	FieldValSize         = 32
	JacobianPointSize    = 33
	AdaptorSignatureSize = JacobianPointSize + ModNScalarSize
)

type AdaptorSignature struct {
	R0 btcec.JacobianPoint
	s  btcec.ModNScalar
}

// newAdaptorSignature creates a new AdaptorSignature with the given parameters.
// It copies the values to avoid unexpected modifications.
func newAdaptorSignature(r *btcec.JacobianPoint, s *btcec.ModNScalar) *AdaptorSignature {
	var sig AdaptorSignature
	sig.R0.Set(r)
	sig.s.Set(s)
	return &sig
}

// NewAdaptorSignatureFromBytes parses the given byte array to an adaptor signature.
// The format is:
// - r (33 bytes): The Jacobian point R
// - s (32 bytes): The scalar s
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

	// extract s
	var s btcec.ModNScalar
	s.SetByteSlice(asigBytes[JacobianPointSize:])

	// Create a new AdaptorSignature
	// Since r is already a pointer, we can use newAdaptorSignature
	return newAdaptorSignature(&r, &s), nil
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
// - s (32 bytes): The scalar s
func (sig *AdaptorSignature) Marshal() ([]byte, error) {
	if sig == nil {
		return nil, nil
	}
	var asigBytes []byte
	// append r
	rBytes := btcec.JacobianToByteSlice(sig.R0)
	asigBytes = append(asigBytes, rBytes...)
	// append s
	sBytes := sig.s.Bytes()
	asigBytes = append(asigBytes, sBytes[:]...)
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
