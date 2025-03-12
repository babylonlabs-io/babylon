package schnorr_adaptor_signature

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cometbft/cometbft/libs/rand"
	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

var AdaptorSignatureTagAux = []byte("SchnorrAdaptor/aux")
var AdaptorSignatureTagNonce = []byte("SchnorrAdaptor/nonce")

// EncVerify verifies that the adaptor signature is valid with respect to the given
// public key, encryption key and message hash.
func (sig *AdaptorSignature) EncVerify(pk *btcec.PublicKey, encKey *EncryptionKey, msgHash []byte) error {
	pkBytes := schnorr.SerializePubKey(pk)
	return encVerify(sig, msgHash, pkBytes, &encKey.JacobianPoint)
}

// Decrypt decrypts the adaptor signature to a Schnorr signature by
// using the decryption key.
func (sig *AdaptorSignature) Decrypt(decKey *DecryptionKey) (*schnorr.Signature, error) {
	return decrypt(sig, &decKey.ModNScalar)
}

// Extract extracts the decryption key from an adaptor signature
// and the Schnorr signature decrypted from it.
func (sig *AdaptorSignature) Extract(decryptedSchnorrSig *schnorr.Signature) (*DecryptionKey, error) {
	scalar, err := extract(sig, decryptedSchnorrSig)
	if err != nil {
		return nil, err
	}

	// Create a DecryptionKey directly without enforcing even encryption key since
	// we are extracting it from an existing adaptor signature that was already
	// verified. The extracted decryption key must be valid since it was used to
	// create a valid Schnorr signature.
	return &DecryptionKey{*scalar}, nil
}

// genNonce generates a nonce for signing according to BIP340 specification with
// domain separation for Schnorr adaptor signatures:
//  5. Let t be the byte-wise XOR of bytes(d) and tagged_hash("SchnorrAdaptor/aux", a)
//     where a is auxiliary random data
//  6. Let rand = tagged_hash("SchnorrAdaptor/nonce", t || T || P || m)
func genRandForNonce(
	skBytes [chainhash.HashSize]byte,
	auxRand []byte,
	encKeyBytes []byte,
	pkBytes []byte,
	msgHash []byte,
) [chainhash.HashSize]byte {
	// Step 5: t = bytes(d) XOR tagged_hash("SchnorrAdaptor/aux", aux)
	auxHash := chainhash.TaggedHash(AdaptorSignatureTagAux, auxRand)
	var t [chainhash.HashSize]byte
	for i := 0; i < chainhash.HashSize; i++ {
		t[i] = skBytes[i] ^ auxHash[i]
	}

	// Step 6: rand = tagged_hash("SchnorrAdaptor/nonce", t || T || P || msg)
	randForNonce := chainhash.TaggedHash(AdaptorSignatureTagNonce, t[:], encKeyBytes, pkBytes, msgHash)
	return *randForNonce
}

// EncSign creates an adaptor signature using the given private key, encryption key,
// and message hash. It generates random auxiliary data internally.
func EncSign(sk *btcec.PrivateKey, encKey *EncryptionKey, msgHash []byte) (*AdaptorSignature, error) {
	auxData := rand.Bytes(chainhash.HashSize)
	return EncSignWithAuxData(sk, encKey, msgHash, auxData)
}

// EncSignWithAuxData creates an adaptor signature using the given private key,
// encryption key, message hash, and auxiliary data.
// allowing the caller to provide auxiliary data for deterministic nonce generation.
func EncSignWithAuxData(sk *btcec.PrivateKey, encKey *EncryptionKey, msgHash []byte, auxData []byte) (*AdaptorSignature, error) {
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

	encKeyBytes := encKey.ToBytes()

	// Steps 5-16: Try to generate adaptor signature with different nonces until successful
	for iteration := uint32(0); ; iteration++ {
		// Step 5-6: Generate random bytes for nonce generation
		// genRandForNonce does the following:
		// - Generates t as byte-wise XOR of bytes(d) and tagged_hash("SchnorrAdaptor/aux", a)
		// - Generates rand = tagged_hash("SchnorrAdaptor/nonce", t || T || P || m)
		var skBytes [chainhash.HashSize]byte
		skScalar.PutBytes(&skBytes)
		randForNonce := genRandForNonce(skBytes, auxData, encKeyBytes, pkBytes, msgHash)

		// Step 7: Generate nonce `k' = int(rand) mod n`
		var nonce btcec.ModNScalar
		nonce.SetBytes(&randForNonce)

		// Step 8: Return FAIL if k' == 0
		if nonce.IsZero() {
			continue
		}

		// Steps 9-16: Generate adaptor signature with the nonce
		adaptorSig, err := encSign(&skScalar, &nonce, pk, msgHash, &encKey.JacobianPoint)
		if err != nil {
			// Try again with a new nonce if this one doesn't work
			continue
		}

		return adaptorSig, nil
	}
}
