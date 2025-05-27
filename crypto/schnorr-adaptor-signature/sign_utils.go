package schnorr_adaptor_signature

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/v4/crypto/common"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// encSign implements the core of the `schnorr_presig_sign` algorithm as defined in the spec.
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
	R.ToAffine()

	// Step 10: Compute R + T
	// Ensure T is in affine coordinates
	tAffine := *t
	tAffine.ToAffine()

	var R0 btcec.JacobianPoint
	btcec.AddNonConst(R, &tAffine, &R0)

	// Ensure R0 is not infinite
	if (R0.X.IsZero() && R0.Y.IsZero()) || R0.Z.IsZero() {
		return nil, fmt.Errorf("R0 point is at infinity")
	}
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
	adaptorSig, err := newAdaptorSignature(&R0, s)
	if err != nil {
		return nil, fmt.Errorf("failed to create adaptor signature: %w", err)
	}

	// Verify the signature
	if err := encVerify(adaptorSig, m, pBytes, t); err != nil {
		return nil, fmt.Errorf("verification of generated signature failed: %w", err)
	}

	return adaptorSig, nil
}

// encVerify implements the `schnorr_presig_verify` algorithm as defined in the spec.
// It verifies that a pre-signature is valid with respect to the given
// public key, encryption key, and message.
func encVerify(
	adaptorSig *AdaptorSignature,
	m []byte,
	pubKeyBytes []byte,
	t *btcec.JacobianPoint,
) error {
	// Check message length
	if len(m) != chainhash.HashSize {
		return fmt.Errorf("wrong size for message (got %v, want %v)",
			len(m), chainhash.HashSize)
	}
	// Check public key length
	if len(pubKeyBytes) != chainhash.HashSize {
		return fmt.Errorf("wrong size for public key (got %v, want %v)",
			len(pubKeyBytes), chainhash.HashSize)
	}

	// Step 1: Let P = lift_x(pubkey)
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return err
	}

	// Step 2: Let s = int(psig[33:65]); return FAIL if s >= n
	s := adaptorSig.s
	if s.IsZero() {
		return fmt.Errorf("s value is zero")
	}

	// Step 3: Let R0 = lift_x(psig[0:33])
	R0 := adaptorSig.R0
	R0.ToAffine()

	// Check if R0 is valid
	if (R0.X.IsZero() && R0.Y.IsZero()) || R0.Z.IsZero() {
		return fmt.Errorf("R0 point is at infinity")
	}

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
	R.ToAffine()

	// Step 6: Let T = R0 + (-R) if has_even_y(R0) else R0 + R
	var T btcec.JacobianPoint
	if !R0.Y.IsOdd() {
		btcec.AddNonConst(&R0, negatePoint(&R), &T)
	} else {
		btcec.AddNonConst(&R0, &R, &T)
	}

	if (T.X.IsZero() && T.Y.IsZero()) || T.Z.IsZero() {
		return fmt.Errorf("T point is at infinity")
	}
	T.ToAffine()

	// Step 7: Verify T matches encryption key
	if !T.X.Equals(&t.X) || !T.Y.Equals(&t.Y) {
		return fmt.Errorf("extracted encryption key does not match")
	}

	return nil
}

// decrypt decrypts a pre-signature using a decryption key.
// This implements the `schnorr_adapt` algorithm as defined in the spec.
func decrypt(psig *AdaptorSignature, dk *btcec.ModNScalar) (*schnorr.Signature, error) {
	// Get s value from adaptor signature
	s0 := psig.s

	// Get decryption key value
	t := *dk

	// Check if s0 or t is zero or exceeds curve order
	if s0.IsZero() || t.IsZero() {
		return nil, fmt.Errorf("s0 or decryption key is zero")
	}

	// Get R point from adaptor signature
	R0 := psig.R0

	// Compute final s value based on R0.Y being even/odd
	// This matches the Python reference implementation:
	// if sig[0] == 2 (even y): s = (s0 + t) % n
	// if sig[0] == 3 (odd y): s = (s0 - t) % n
	var s btcec.ModNScalar
	if !R0.Y.IsOdd() {
		// R0.Y is even (sig[0] == 2)
		// s = (s0 + t) % n
		s.Set(&s0)
		s.Add(&t)
	} else {
		// R0.Y is odd (sig[0] == 3)
		// s = (s0 - t) % n
		s.Set(&s0)
		tNeg := t
		tNeg.Negate()
		s.Add(&tNeg)
	}

	// Create Schnorr signature from R0.X and s
	return schnorr.NewSignature(&R0.X, &s), nil
}

// extract extracts the decryption key from a pre-signature and its decrypted signature.
// This implements the `schnorr_extract_secadaptor` algorithm as defined in the spec.
func extract(psig *AdaptorSignature, sig *schnorr.Signature) (*btcec.ModNScalar, error) {
	// Get s0 value from adaptor signature
	s0 := psig.s

	// Get s value from schnorr signature
	sigBytes := sig.Serialize()
	sBytes := sigBytes[32:]
	var s btcec.ModNScalar
	if overflow := s.SetByteSlice(sBytes); overflow {
		return nil, fmt.Errorf("s value in signature is invalid")
	}

	// Check if s0 or s is zero or exceeds curve order
	if s0.IsZero() || s.IsZero() {
		return nil, fmt.Errorf("s values must be non-zero")
	}

	// Get R0 point from adaptor signature to check its Y parity
	R0 := psig.R0

	// Calculate decryption key based on R0.Y being even/odd
	// This matches the Python reference implementation:
	// if sig65[0] == 2 (even y): t = (s - s0) % n
	// if sig65[0] == 3 (odd y): t = (s0 - s) % n
	var dk btcec.ModNScalar
	if !R0.Y.IsOdd() {
		// R0.Y is even (sig65[0] == 2)
		// t = (s - s0) % n
		dk.Set(&s)
		s0Neg := new(btcec.ModNScalar).Set(&s0)
		s0Neg.Negate()
		dk.Add(s0Neg)
	} else {
		// R0.Y is odd (sig65[0] == 3)
		// t = (s0 - s) % n
		dk.Set(&s0)
		sNeg := new(btcec.ModNScalar).Set(&s)
		sNeg.Negate()
		dk.Add(sNeg)
	}

	return &dk, nil
}

// negatePoint negates a point (either Jacobian or affine)
// It returns a new point with the same x-coordinate but negated y-coordinate.
func negatePoint(point *btcec.JacobianPoint) *btcec.JacobianPoint {
	nPoint := *point
	nPoint.Y.Negate(1).Normalize()
	return &nPoint
}
