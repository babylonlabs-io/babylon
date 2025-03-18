package schnorr_adaptor_signature

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

const (
	ModNScalarSize                = 32
	FieldValSize                  = 32
	JacobianPointSize             = 33
	AdaptorSignatureSize          = JacobianPointSize + ModNScalarSize
	AdaptorSignatureSizeOldFormat = AdaptorSignatureSize + 1
)

type AdaptorSignature struct {
	R0 btcec.JacobianPoint
	s  btcec.ModNScalar
}

// newAdaptorSignature creates a new AdaptorSignature with the given parameters.
// It copies the values to avoid unexpected modifications.
func newAdaptorSignature(r *btcec.JacobianPoint, s *btcec.ModNScalar) (*AdaptorSignature, error) {
	var sig AdaptorSignature

	// Ensure r is an affine point
	if r.Z.IsZero() || !r.Z.IsOne() {
		return nil, fmt.Errorf("R0 must be an affine point")
	}

	sig.R0.Set(r)
	sig.s.Set(s)
	return &sig, nil
}

// convertOldFormatToNewFormat converts the old format of
// adaptor signature to the new format.
func convertOldFormatToNewFormat(oldSigBytes []byte) ([]byte, error) {
	if oldSigBytes[0] != 0x02 {
		return nil, fmt.Errorf("invalid point (must be even)")
	}

	var newSigBytes [AdaptorSignatureSize]byte

	switch oldSigBytes[AdaptorSignatureSizeOldFormat-1] {
	case 0x00:
		newSigBytes[0] = 0x02
	case 0x01:
		newSigBytes[0] = 0x03
	default:
		return nil, fmt.Errorf("invalid needsNegation byte")
	}

	copy(newSigBytes[1:], oldSigBytes[1:AdaptorSignatureSizeOldFormat-1])

	return newSigBytes[:], nil
}

// NewAdaptorSignatureFromBytes parses the given byte array to an adaptor signature.
// It handles two formats:
// The new format is:
// - First 32 bytes: R0.X
// - Last 32 bytes: s scalar
// The old format is:
// - First 32 bytes: R0.X
// - Next 32 bytes: s scalar
// - Last byte: bool on needNegation
func NewAdaptorSignatureFromBytes(rawBytes []byte) (*AdaptorSignature, error) {
	var asigBytes []byte

	switch len(rawBytes) {
	case AdaptorSignatureSizeOldFormat:
		// old format, convert to new format
		newSigBytes, err := convertOldFormatToNewFormat(rawBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert old format to new format: %w", err)
		}
		asigBytes = newSigBytes
	case AdaptorSignatureSize:
		// new format, copy
		asigBytes = rawBytes
	default:
		return nil, fmt.Errorf(
			"the length of the given bytes for adaptor signature is incorrect (expected: %d or %d, actual: %d)",
			AdaptorSignatureSize,
			AdaptorSignatureSizeOldFormat,
			len(rawBytes),
		)
	}

	r0, err := btcec.ParseJacobian(asigBytes[:33])
	if err != nil {
		return nil, fmt.Errorf("failed to parse R0: %w", err)
	}

	// Extract s from bytes 33-64
	var s btcec.ModNScalar
	if overflow := s.SetByteSlice(asigBytes[33:65]); overflow {
		return nil, fmt.Errorf("s value is invalid")
	}

	// Create a new AdaptorSignature
	return newAdaptorSignature(&r0, &s)
}

// NewAdaptorSignatureFromHex parses the given hex string to an adaptor signature.
func NewAdaptorSignatureFromHex(asigHex string) (*AdaptorSignature, error) {
	asigBytes, err := hex.DecodeString(asigHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	return NewAdaptorSignatureFromBytes(asigBytes)
}

// Marshal serializes an adaptor signature to bytes.
// The format is:
// [0]: parity byte (0x02 for even Y, 0x03 for odd Y)
// [1:33]: X coordinate of R0
// [33:65]: s value
func (a *AdaptorSignature) Marshal() ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("adaptor signature is nil")
	}

	// Ensure R0 is in affine coordinates
	R0 := a.R0
	R0.ToAffine()

	// Create the result buffer
	result := make([]byte, 65)

	// Set the parity byte based on R0.Y
	if R0.Y.IsOdd() {
		result[0] = 0x03
	} else {
		result[0] = 0x02
	}

	// Copy R0.X to result[1:33]
	R0.X.PutBytesUnchecked(result[1:33])

	// Copy s to result[33:65]
	var sBytes [32]byte
	a.s.PutBytesUnchecked(sBytes[:])
	copy(result[33:], sBytes[:])

	return result, nil
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
