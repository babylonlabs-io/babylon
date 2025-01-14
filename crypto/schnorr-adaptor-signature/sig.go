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

// AdaptorSignature is the structure for an adaptor signature
// the adaptor signature is a triple (R, s', need_negation) where
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

func newAdaptorSignature(r *btcec.JacobianPoint, sHat *btcec.ModNScalar, needNegation bool) *AdaptorSignature {
	var sig AdaptorSignature
	sig.r.Set(r)
	sig.sHat.Set(sHat)
	sig.needNegation = needNegation
	return &sig
}

// EncVerify verifies that the adaptor signature is valid w.r.t. the given
// public key, encryption key and message hash
func (sig *AdaptorSignature) EncVerify(pk *btcec.PublicKey, encKey *EncryptionKey, msgHash []byte) error {
	pkBytes := schnorr.SerializePubKey(pk)
	return encVerify(sig, msgHash, pkBytes, &encKey.JacobianPoint)
}

// Decrypt decrypts the adaptor signature to a Schnorr signature by
// using the decryption key `decKey`, noted by `t` in the paper
func (sig *AdaptorSignature) Decrypt(decKey *DecryptionKey) *schnorr.Signature {
	R := sig.r

	t := decKey.ModNScalar
	if sig.needNegation {
		t.Negate()
	}
	// s = s' + t (or s'-t if negation is needed)
	s := sig.sHat
	s.Add(&t)

	return schnorr.NewSignature(&R.X, &s)
}

// Recover recovers the decryption key by using the adaptor signature
// and the Schnorr signature decrypted from it
func (sig *AdaptorSignature) Recover(decryptedSchnorrSig *schnorr.Signature) *DecryptionKey {
	// unpack s and R from Schnorr signature
	_, s := unpackSchnorrSig(decryptedSchnorrSig)
	sHat := sig.sHat

	// extract encryption key t = s - s'
	sHat.Negate()
	t := s.Add(&sHat)

	if sig.needNegation {
		t.Negate()
	}

	return &DecryptionKey{*t}
}

// Marshal is to implement proto interface
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

func (sig *AdaptorSignature) MarshalHex() string {
	return hex.EncodeToString(sig.MustMarshal())
}

// Size is to implement proto interface
func (sig *AdaptorSignature) Size() int {
	return AdaptorSignatureSize
}

// MarshalTo is to implement proto interface
func (sig *AdaptorSignature) MarshalTo(data []byte) (int, error) {
	bz, err := sig.Marshal()
	if err != nil {
		return 0, err
	}
	copy(data, bz)
	return len(data), nil
}

// Unmarshal is to implement proto interface
func (sig *AdaptorSignature) Unmarshal(data []byte) error {
	adaptorSig, err := NewAdaptorSignatureFromBytes(data)
	if err != nil {
		return err
	}

	*sig = *adaptorSig

	return nil
}

func (sig *AdaptorSignature) Equals(sig2 AdaptorSignature) bool {
	return bytes.Equal(sig.MustMarshal(), sig2.MustMarshal())
}

// appendAndHash appends the given data and hashes the result
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
// encryption key (noted by `T` in the paper) and message hash
func EncSign(sk *btcec.PrivateKey, encKey *EncryptionKey, msgHash []byte) (*AdaptorSignature, error) {
	// d' = int(d)
	var skScalar btcec.ModNScalar
	skScalar.Set(&sk.Key)

	// Fail if msgHash is not 32 bytes
	if len(msgHash) != chainhash.HashSize {
		return nil, fmt.Errorf("wrong size for message hash (got %v, want %v)", len(msgHash), chainhash.HashSize)
	}

	// Fail if d = 0 or d >= n
	if skScalar.IsZero() {
		return nil, fmt.Errorf("private key is zero")
	}

	// P = 'd*G
	pk := sk.PubKey()

	// Negate d if P.y is odd.
	pubKeyBytes := pk.SerializeCompressed()
	if pubKeyBytes[0] == secp.PubKeyFormatCompressedOdd {
		skScalar.Negate()
	}

	var privKeyBytes [chainhash.HashSize]byte
	skScalar.PutBytes(&privKeyBytes)

	encKeyBytes := encKey.ToBTCPK().SerializeCompressed()
	// hashForNonce is sha256(m || P || T)
	hashForNonce := appendAndHash(msgHash, pubKeyBytes, encKeyBytes)

	for iteration := uint32(0); ; iteration++ {
		// Use RFC6979 to generate a deterministic nonce in [1, n-1]
		// parameterized by the private key, message being signed, extra data
		// that identifies the scheme, and an iteration count
		nonce := btcec.NonceRFC6979(
			privKeyBytes[:], hashForNonce, customBabylonRFC6979ExtraDataV0[:], nil, iteration,
		)

		// try to generate adaptor signature
		sig, err := encSign(&skScalar, nonce, pk, msgHash, &encKey.JacobianPoint)
		if err != nil {
			// Try again with a new nonce.
			continue
		}

		return sig, nil
	}
}

// NewAdaptorSignatureFromBytes parses the given byte array to an adaptor signature
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
		return nil, err
	}
	// extract sHat
	var sHat btcec.ModNScalar
	sHat.SetByteSlice(asigBytes[JacobianPointSize : JacobianPointSize+ModNScalarSize])
	// extract needNegation
	needNegation := asigBytes[AdaptorSignatureSize-1] != 0x00

	return newAdaptorSignature(&r, &sHat, needNegation), nil
}

// NewAdaptorSignatureFromHex parses the given hex string to an adaptor signature
func NewAdaptorSignatureFromHex(asigHex string) (*AdaptorSignature, error) {
	asigBytes, err := hex.DecodeString(asigHex)
	if err != nil {
		return nil, err
	}
	return NewAdaptorSignatureFromBytes(asigBytes)
}
