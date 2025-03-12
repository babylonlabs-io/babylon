package schnorr_adaptor_signature

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/crypto/common"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// encSign implements the core of the EncSign algorithm as defined in the spec.
// It generates a 65-byte pre-signature using the given private key, nonce,
// public key, message, and encryption key.
//
// The algorithm starts from step 9 since steps 1-8 are handled in EncSign
func encSign(
	privKey, nonce *btcec.ModNScalar,
	pubKey *btcec.PublicKey,
	m []byte,
	t *btcec.JacobianPoint,
) (*AdaptorSignature, error) {
	// Step 9: Compute R = k0*G
	// NOTE: later we will negate k0 if needed, and after that k = k0
	k := *nonce
	R, err := common.ScalarBaseMultWithBlinding(&k)
	if err != nil {
		return nil, fmt.Errorf("failed to compute kG: %w", err)
	}

	// Step 10: Compute R + T
	Tp, err := liftX(&t.X)
	if err != nil {
		return nil, fmt.Errorf("failed to lift t: %w", err)
	}

	var R0 btcec.JacobianPoint
	btcec.AddNonConst(R, Tp, &R0)
	R0.ToAffine()

	// If R0 has odd y, negate k
	if R0.Y.IsOdd() {
		k.Negate()
	}

	// Compute challenge e = tagged_hash("BIP0340/challenge", bytes(R0) || bytes(P) || m)
	var r0Bytes [chainhash.HashSize]byte
	R0.X.PutBytesUnchecked(r0Bytes[:])
	pBytes := schnorr.SerializePubKey(pubKey)
	commitment := chainhash.TaggedHash(
		chainhash.TagBIP0340Challenge, r0Bytes[:], pBytes, m,
	)
	var e btcec.ModNScalar
	e.SetBytes((*[ModNScalarSize]byte)(commitment))

	// Compute s = k + e*d mod n
	s := new(btcec.ModNScalar).Mul2(&e, privKey).Add(&k)

	// Create 65-byte pre-signature
	adaptorSig := newAdaptorSignature(&R0, s)

	// Verify the signature
	if err := encVerify(adaptorSig, m, pBytes, t); err != nil {
		return nil, fmt.Errorf("verification of generated signature failed: %w", err)
	}

	return adaptorSig, nil
}

// encVerify implements the EncVerify algorithm as defined in the spec.
// It verifies that a pre-signature is valid with respect to the given
// public key, encryption key, and message.
func encVerify(
	adaptorSig *AdaptorSignature,
	m []byte,
	pubKeyBytes []byte,
	t *btcec.JacobianPoint,
) error {
	if len(m) != chainhash.HashSize {
		return fmt.Errorf("wrong size for message (got %v, want %v)",
			len(m), chainhash.HashSize)
	}
	if len(pubKeyBytes) != chainhash.HashSize {
		return fmt.Errorf("wrong size for public key (got %v, want %v)",
			len(pubKeyBytes), chainhash.HashSize)
	}

	// Step 1: Let P = lift_x(pubkey)
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return err
	}
	if !pubKey.IsOnCurve() {
		return fmt.Errorf("pubkey point is not on curve")
	}

	// Step 2: Let s = int(psig[33:65]); return FAIL if s >= n
	s := adaptorSig.s

	// Step 3: Let R0 = lift_x(psig[0:33])
	R0 := adaptorSig.R0

	// Step 4: Compute challenge e = H(R0.x || P || m)
	var r0Bytes [chainhash.HashSize]byte
	R0.X.PutBytesUnchecked(r0Bytes[:])
	commitment := chainhash.TaggedHash(
		chainhash.TagBIP0340Challenge, r0Bytes[:], pubKeyBytes, m,
	)
	var e btcec.ModNScalar
	e.SetBytes((*[ModNScalarSize]byte)(commitment))

	// Step 5: Compute R = s*G - e*P
	var P, R, sG, eP btcec.JacobianPoint
	pubKey.AsJacobian(&P)
	btcec.ScalarBaseMultNonConst(&s, &sG)
	e.Negate() // Negate e to use AddNonConst for subtraction
	btcec.ScalarMultNonConst(&e, &P, &eP)
	btcec.AddNonConst(&sG, &eP, &R)

	if (R.X.IsZero() && R.Y.IsZero()) || R.Z.IsZero() {
		return fmt.Errorf("R point is at infinity")
	}

	// Step 6: Let T = R0 + (-R) if has_even_y(R0) else R0 + R
	R.ToAffine()
	var T btcec.JacobianPoint
	if !R0.Y.IsOdd() {
		btcec.AddNonConst(&R0, negatePoint(&R), &T)
	} else {
		btcec.AddNonConst(&R0, &R, &T)
	}

	if (T.X.IsZero() && T.Y.IsZero()) || T.Z.IsZero() {
		return fmt.Errorf("T point is at infinity")
	}

	// Step 7: Verify T matches encryption key
	T.ToAffine()
	if !T.X.Equals(&t.X) || !T.Y.Equals(&t.Y) {
		return fmt.Errorf("extracted encryption key does not match")
	}

	return nil
}

// decrypt decrypts a pre-signature using a decryption key.
// This implements the Decrypt algorithm as defined in the spec.
func decrypt(psig *AdaptorSignature, dk *DecryptionKey) (*schnorr.Signature, error) {
	// Get s value from adaptor signature
	s0 := psig.s

	// Get decryption key value
	t := dk.ModNScalar

	// Check if s0 or t is zero or exceeds curve order
	if s0.IsZero() || t.IsZero() {
		return nil, fmt.Errorf("s0 or decryption key is zero")
	}

	// Get R point from adaptor signature
	R := psig.R0
	R.ToAffine()

	// Compute final s value based on R.Y being even/odd
	var s btcec.ModNScalar
	if !R.Y.IsOdd() {
		// R.Y is even, so s = s0 + t
		s.Set(&s0)
		s.Add(&t)
	} else {
		// R.Y is odd, so s = s0 - t
		s.Set(&s0)
		t.Negate()
		s.Add(&t)
	}

	// Create Schnorr signature from R.X and s
	return schnorr.NewSignature(&R.X, &s), nil
}

// unwrapSchnorrSignature extracts the R point and s scalar bytes from a Schnorr signature.
// Returns the first 32 bytes as R and last 32 bytes as s.
func unwrapSchnorrSignature(sig *schnorr.Signature) ([]byte, []byte) {
	sigBytes := sig.Serialize()
	return sigBytes[:32], sigBytes[32:]
}

// extract extracts the decryption key from a pre-signature and its decrypted signature.
// This implements the Extract algorithm as defined in the spec.
func extract(psig *AdaptorSignature, sig *schnorr.Signature) (*DecryptionKey, error) {
	// Get s0 value from adaptor signature
	s0 := psig.s

	// get s value from schnorr signature
	_, sBytes := unwrapSchnorrSignature(sig)
	s := new(btcec.ModNScalar)
	s.SetByteSlice(sBytes)
	if s0.IsZero() || s.IsZero() {
		return nil, fmt.Errorf("s values must be non-zero")
	}

	// Get R point from adaptor signature and convert to affine
	R := psig.R0
	R.ToAffine()

	// Calculate decryption key based on R.Y being even/odd
	var dk btcec.ModNScalar
	if !R.Y.IsOdd() {
		// R.Y is even, so dk = s - s0
		dk.Set(s)
		s0.Negate()
		dk.Add(&s0)
	} else {
		// R.Y is odd, so dk = s0 - s
		dk.Set(&s0)
		s.Negate()
		dk.Add(s)
	}

	return NewDecryptionKeyFromModNScalar(&dk)
}

// liftX lifts an x-coordinate to a point on the curve with even y-coordinate.
// It returns a pointer to a JacobianPoint or an error if lifting fails.
func liftX(x *btcec.FieldVal) (*btcec.JacobianPoint, error) {
	var y btcec.FieldVal
	if success := btcec.DecompressY(x, false, &y); !success {
		return nil, fmt.Errorf("failed to decompress y")
	}
	var z btcec.FieldVal
	z.SetInt(1)
	point := btcec.MakeJacobianPoint(x, &y, &z)
	return &point, nil
}

// negatePoint negates a point (either Jacobian or affine)
// It returns a new point with the same x-coordinate but negated y-coordinate.
func negatePoint(point *btcec.JacobianPoint) *btcec.JacobianPoint {
	nPoint := *point
	nPoint.Y.Negate(1).Normalize()
	return &nPoint
}
