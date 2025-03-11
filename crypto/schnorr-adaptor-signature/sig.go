// This file provides the high-level API wrappers around the core cryptographic
// operations implemented in sign_utils.go, which implements the ./spec.md.
//
// The scheme consists of four main algorithms:
// - EncSign: Creates a pre-signature using a private key and encryption key
// - EncVerify: Verifies a pre-signature using a public key and encryption key
// - Decrypt: Decrypts a pre-signature using a decryption key to obtain a valid Schnorr signature
// - Extract: Extracts the decryption key from a pre-signature and its corresponding Schnorr signature
//
// See sign_utils.go for the detailed implementation and ./spec.md for the protocol specification
// and security properties.

package schnorr_adaptor_signature

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cometbft/cometbft/libs/rand"
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
	return encVerify(sig.ToSpecBytes(), msgHash, pkBytes, &encKey.FieldVal)
}

// Decrypt decrypts the adaptor signature to a Schnorr signature by
// using the decryption key.
func (sig *AdaptorSignature) Decrypt(decKey *DecryptionKey) (*schnorr.Signature, error) {
	psig := sig.ToSpecBytes()
	decryptedSchnorrSig, err := decrypt(psig, decKey.ToBytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt adaptor signature: %w", err)
	}
	return schnorr.ParseSignature(decryptedSchnorrSig)
}

// Extract extracts the decryption key by using the adaptor signature
// and the Schnorr signature decrypted from it.
func (sig *AdaptorSignature) Extract(decryptedSchnorrSig *schnorr.Signature) (*DecryptionKey, error) {
	psig := sig.ToSpecBytes()

	// unpack s and R from Schnorr signature
	sigBytes := decryptedSchnorrSig.Serialize()

	dkBytes, err := extract(psig, sigBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to extract decryption key: %w", err)
	}

	dk, err := NewDecryptionKeyFromBytes(dkBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryption key: %w", err)
	}

	return dk, nil
}

// genNonce generates a nonce for signing according to BIP340 specification:
//  5. Let t be the byte-wise XOR of bytes(d) and tagged_hash("BIP0340/aux", a)
//     where a is auxiliary random data
//  6. Let rand = tagged_hash("BIP0340/nonce", t || bytes(Pp) || m)
//
// The nonce generation follows BIP340 with additional domain separation for
// Babylon adaptor signatures.
func genRandForNonce(
	skBytes [chainhash.HashSize]byte,
	auxRand []byte,
	pkBytes []byte,
	msgHash []byte,
) [chainhash.HashSize]byte {
	// Step 5: t = bytes(d) XOR tagged_hash("BIP0340/aux", a)
	auxHash := chainhash.TaggedHash(chainhash.TagBIP0340Aux, auxRand)
	var t [chainhash.HashSize]byte
	for i := 0; i < chainhash.HashSize; i++ {
		t[i] = skBytes[i] ^ auxHash[i]
	}

	// Step 6: rand = tagged_hash("BIP0340/nonce", t || bytes(Pp) || m)
	randForNonce := chainhash.TaggedHash(chainhash.TagBIP0340Nonce, t[:], pkBytes, msgHash)
	return *randForNonce
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
	pkBytes := pk.SerializeCompressed()
	if pkBytes[0] == secp.PubKeyFormatCompressedOdd {
		skScalar.Negate()
	}

	// Steps 5-16: Try to generate adaptor signature with different nonces until successful
	for iteration := uint32(0); ; iteration++ {
		// Step 5-6: Generate random bytes for nonce generation
		// genRandForNonce does the following:
		// 1. Generates t as byte-wise XOR of bytes(d) and tagged_hash("BIP0340/aux", a)
		// 2. Generates rand = tagged_hash("BIP0340/nonce", t || bytes(Pp) || m)
		var skBytes [chainhash.HashSize]byte
		skScalar.PutBytes(&skBytes)
		auxData := rand.Bytes(chainhash.HashSize)
		randForNonce := genRandForNonce(skBytes, auxData, pkBytes, msgHash)

		// Step 7: Generate nonce `k' = int(rand) mod n`
		var nonce btcec.ModNScalar
		nonce.SetBytes(&randForNonce)

		// Step 8: Return FAIL if k' == 0
		if nonce.IsZero() {
			continue
		}

		// Steps 9-16: Generate adaptor signature with the nonce
		psig, err := encSign(&skScalar, &nonce, pk, msgHash, &encKey.FieldVal)
		if err != nil {
			// Try again with a new nonce if this one doesn't work
			continue
		}

		return NewAdaptorSignatureFromSpecFormat(psig, encKey.ToBytes())
	}
}
