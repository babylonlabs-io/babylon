package schnorr_adaptor_signature

import (
	"crypto/sha256"
	"fmt"

	"github.com/babylonlabs-io/babylon/crypto/common"
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

// EncVerify verifies that the adaptor signature is valid with respect to the given
// public key, encryption key and message hash.
func (sig *AdaptorSignature) EncVerify(pk *btcec.PublicKey, encKey *EncryptionKey, msgHash []byte) error {
	pkBytes := schnorr.SerializePubKey(pk)
	return encVerify(sig.ToPreSignature(), msgHash, pkBytes, &encKey.FieldVal)
}

// Decrypt decrypts the adaptor signature to a Schnorr signature by
// using the decryption key.
func (sig *AdaptorSignature) Decrypt(decKey *DecryptionKey) *schnorr.Signature {
	// Step 1-2: Extract Rp and s from the adaptor signature
	Rp := sig.r
	s := sig.sHat

	// Step 3-4: Extract decryption key
	u := decKey.ModNScalar

	// Step 5: Compute T' = u' * G
	T, err := common.ScalarBaseMultWithBlinding(&u)
	if err != nil {
		// This should never happen with a valid decryption key
		panic("failed to compute T = u*G")
	}
	T.ToAffine()

	// Step 6: Ensure T has even y-coordinate
	var Tp btcec.JacobianPoint
	Tp = *T
	var actualU btcec.ModNScalar
	actualU.Set(&u)
	if T.Y.IsOdd() {
		// If T.y is odd, negate both T and u
		Tp.Y.Negate(1).Normalize()
		actualU.Negate()
	}

	// Step 7: Compute ss and RTp
	var RTp btcec.JacobianPoint
	var ss btcec.ModNScalar

	// First try: Rp + Tp
	if !sig.needNegation {
		btcec.AddNonConst(&Rp, &Tp, &RTp)
		RTp.ToAffine()
		ss.Set(&s)
		ss.Add(&actualU)
	} else {
		// Second try: Rp - Tp
		var negTp btcec.JacobianPoint
		negTp = Tp
		negTp.Y.Negate(1).Normalize()
		btcec.AddNonConst(&Rp, &negTp, &RTp)
		RTp.ToAffine()
		ss.Set(&s)
		var negU btcec.ModNScalar
		negU.Set(&actualU)
		negU.Negate()
		ss.Add(&negU)
	}

	// Ensure RTp has even y-coordinate as required by BIP-340
	if RTp.Y.IsOdd() {
		RTp.Y.Negate(1).Normalize()
		ss.Negate()
	}

	// Create and return the signature
	return schnorr.NewSignature(&RTp.X, &ss)
}

// Recover recovers the decryption key by using the adaptor signature
// and the Schnorr signature decrypted from it.
//
// This implements the Extract algorithm as defined in the spec:
// 1. Let ss = int(sig[32:64]) - extract s from the decrypted Schnorr signature
// 2. Let s = int(psig[32:64]) - extract s' from the adaptor signature
// 3. Let dk' = (ss - s) mod n - compute the decryption key
// 4. Return FAIL if ek != bytes(int(dk') * G)
// 5. Return dk'
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
