package schnorr_adaptor_signature

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

var (
	// CustomBabylonrfc6979ExtraDataV0 is the extra data to feed to RFC6979 when
	// generating the deterministic nonce for the BIP-340 Babylon adaptor signature scheme.
	// This ensures the same nonce is not generated for the same message and key
	// as for other signing algorithms such as ECDSA.
	//
	// It is equal to SHA-256([]byte("BIP-340/babylon-adaptor-signature")).
	customBabylonRFC6979ExtraDataV0 = [chainhash.HashSize]uint8{
		0xcd, 0x36, 0xb5, 0x97, 0xbd, 0x59, 0x08, 0xfc,
		0x48, 0x5c, 0xe9, 0xa2, 0xc0, 0xc2, 0x8b, 0xce,
		0xd0, 0xda, 0xdb, 0x7f, 0xac, 0x7b, 0xf9, 0x4c,
		0x19, 0x68, 0x51, 0xfb, 0x23, 0x27, 0x07, 0x09,
	}
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

// EncVerify verifies that the adaptor signature is valid with respect to the given
// public key, encryption key and message hash.
func (sig *AdaptorSignature) EncVerify(pk *btcec.PublicKey, encKey *EncryptionKey, msgHash []byte) error {
	pkBytes := schnorr.SerializePubKey(pk)
	return encVerify(sig, msgHash, pkBytes, &encKey.FieldVal)
}

// Decrypt decrypts the adaptor signature to a Schnorr signature by
// using the decryption key.
func (sig *AdaptorSignature) Decrypt(decKey *DecryptionKey) *schnorr.Signature {
	// Step 1: Extract R and s' from the adaptor signature
	R := sig.r
	sHat := sig.sHat

	// Steps 2-4: Extract decryption key and apply negation if needed
	// In our implementation, we use the needNegation flag to determine
	// whether the decryption key needs to be negated
	t := decKey.ModNScalar
	if sig.needNegation {
		t.Negate()
	}

	// Step 5: Compute s = s' + u (or s' - u if negation is needed)
	var s btcec.ModNScalar
	s.Set(&sHat)
	s.Add(&t)

	// Step 6: Return the Schnorr signature (R, s)
	return schnorr.NewSignature(&R.X, &s)
}

// Recover recovers the decryption key by using the adaptor signature
// and the Schnorr signature decrypted from it.
func (sig *AdaptorSignature) Recover(decryptedSchnorrSig *schnorr.Signature) *DecryptionKey {
	// Step 1: Extract s from the decrypted Schnorr signature
	_, s := unpackSchnorrSig(decryptedSchnorrSig)

	// Step 2: Extract s' from the adaptor signature
	sHat := sig.sHat

	// Step 3: Compute dk' = (s - s') mod n
	// First negate s' to compute s - s' as s + (-s')
	var negatedSHat btcec.ModNScalar
	negatedSHat.Set(&sHat)
	negatedSHat.Negate()

	// Compute t = s + (-s')
	var t btcec.ModNScalar
	t.Set(s)
	t.Add(&negatedSHat)

	// Step 4: Apply negation if needed based on the adaptor signature
	if sig.needNegation {
		t.Negate()
	}

	// Step 5: Return the decryption key
	return &DecryptionKey{t}
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

// appendAndHash appends the given data and hashes the result.
// This is used in the nonce generation process for EncSign.
// Expected input is:
//   - msgHash: 32 bytes
//   - signerPubKeyBytes: 33 bytes
//   - encKeyBytes: 33 bytes
//
// The output is 32 bytes and is result of sha256(m || P || T)
func appendAndHash(
	msgHash []byte,
	signerPubKeyBytes []byte,
	encKeyBytes []byte,
) []byte {
	combinedData := make([]byte, 98)
	copy(combinedData[0:32], msgHash)
	copy(combinedData[32:65], signerPubKeyBytes)
	copy(combinedData[65:98], encKeyBytes)
	hash := sha256.Sum256(combinedData)
	return hash[:]
}

// EncSign generates an adaptor signature by using the given secret key,
// encryption key (noted by `T` in the paper) and message hash.
func EncSign(sk *btcec.PrivateKey, encKey *EncryptionKey, msgHash []byte) (*AdaptorSignature, error) {
	// Fail if msgHash is not 32 bytes
	if len(msgHash) != chainhash.HashSize {
		return nil, fmt.Errorf("wrong size for message hash (got %v, want %v)", len(msgHash), chainhash.HashSize)
	}

	// Step 1: Let d' = int(d)
	var skScalar btcec.ModNScalar
	skScalar.Set(&sk.Key)

	// Step 2: Return FAIL if d' == 0 or d' >= n
	if skScalar.IsZero() {
		return nil, fmt.Errorf("private key is zero")
	}

	// Step 3: Let Pp = d' * G
	pk := sk.PubKey()

	// Step 4: Let d = d' if has_even_y(Pp), otherwise let d = n - d'
	pubKeyBytes := pk.SerializeCompressed()
	if pubKeyBytes[0] == secp.PubKeyFormatCompressedOdd {
		skScalar.Negate()
	}

	// Step 5: Generate t as byte-wise XOR of bytes(d) and tagged_hash("BIP0340/aux", a)
	// Note: In our implementation, we use RFC6979 for deterministic nonce generation
	var privKeyBytes [chainhash.HashSize]byte
	skScalar.PutBytes(&privKeyBytes)

	// Get encryption key bytes for nonce generation
	encKeyBTCPK, err := encKey.ToBTCPK()
	if err != nil {
		return nil, err
	}
	encKeyBytes := encKeyBTCPK.SerializeCompressed()

	// Step 6-7: Generate rand = tagged_hash("BIP0340/nonce", t || bytes(Pp) || m)
	// We use appendAndHash which combines the message, public key, and encryption key
	hashForNonce := appendAndHash(msgHash, pubKeyBytes, encKeyBytes)

	// Steps 8-16: Try to generate adaptor signature with different nonces until successful
	for iteration := uint32(0); ; iteration++ {
		// Step 8-9: Generate nonce k' and ensure it's not zero
		nonce := btcec.NonceRFC6979(
			privKeyBytes[:], hashForNonce, customBabylonRFC6979ExtraDataV0[:], nil, iteration,
		)

		// Steps 10-16: Generate adaptor signature with the nonce
		sig, err := encSign(&skScalar, nonce, pk, msgHash, &encKey.FieldVal)
		if err != nil {
			// Try again with a new nonce if this one doesn't work
			continue
		}

		return sig, nil
	}
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
