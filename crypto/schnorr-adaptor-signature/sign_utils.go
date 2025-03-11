package schnorr_adaptor_signature

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/crypto/common"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

const (
	ModNScalarSize       = 32
	FieldValSize         = 32
	JacobianPointSize    = 33
	AdaptorSignatureSize = JacobianPointSize + ModNScalarSize + 1
)

// encSign implements the core of the EncSign algorithm as defined in the spec.
// It generates a 64-byte pre-signature using the given private key, nonce,
// public key, message, and encryption key.
//
// The algorithm starts from step 9 since steps 1-8 are handled in EncSign
func encSign(
	privKey, nonce *btcec.ModNScalar,
	pubKey *btcec.PublicKey,
	m []byte,
	t *btcec.FieldVal,
) ([]byte, error) {
	// Step 9: Compute R' = k'*G (with blinding to prevent timing side channel attacks)
	k := *nonce
	RHat, err := common.ScalarBaseMultWithBlinding(&k)
	if err != nil {
		return nil, fmt.Errorf("failed to compute kG: %w", err)
	}

	// Step 10: Let (k, Rp) = (k', R') if has_even_y(R'), otherwise let (k, Rp) = (n - k', -R')
	RHat.ToAffine()
	if RHat.Y.IsOdd() {
		k.Negate()
		// Negate the y-coordinate in place
		RHat.Y.Negate(1).Normalize()
	}

	// Step 11: Let Tp = lift_x(int(ek)); return FAIL if that fails
	Tp, err := liftX(t)
	if err != nil {
		return nil, fmt.Errorf("failed to lift t: %w", err)
	}

	// Step 12: Compute RTp with even y-coordinate
	// If has_even_y(Rp + Tp), let RTp = Rp + Tp
	// Else if has_even_y(Rp - Tp), let RTp = Rp - Tp
	// Else let a = bytes(k) and go back to Step 5
	var RTp btcec.JacobianPoint

	// Try Rp + Tp first
	btcec.AddNonConst(RHat, Tp, &RTp)
	RTp.ToAffine()

	if RTp.Y.IsOdd() {
		// If Rp+Tp has odd y, try Rp-Tp
		btcec.AddNonConst(RHat, negatePoint(Tp), &RTp)
		RTp.ToAffine()

		if RTp.Y.IsOdd() {
			// If both Rp+Tp and Rp-Tp have odd y, we need a new nonce
			// This corresponds to "let a = bytes(k) and go back to Step 5" in the spec
			return nil, fmt.Errorf("both Rp+Tp and Rp-Tp have odd y, need to try again with a new nonce")
		}
	}

	// Step 13: Compute the challenge
	// e = int(tagged_hash("BIP0340/challenge", bytes(RTp) || bytes(Pp) || m)) mod n
	var rtpBytes [chainhash.HashSize]byte
	RTp.X.PutBytesUnchecked(rtpBytes[:])
	pBytes := schnorr.SerializePubKey(pubKey)
	commitment := chainhash.TaggedHash(
		chainhash.TagBIP0340Challenge, rtpBytes[:], pBytes, m,
	)
	var e btcec.ModNScalar
	e.SetBytes((*[ModNScalarSize]byte)(commitment))

	// Step 14: Compute s' = (k + e*d) mod n
	sHat := new(btcec.ModNScalar).Mul2(&e, privKey).Add(&k)

	// Step 15: Create the 64-byte pre-signature (bytes(Rp) || bytes(s'))
	presig := make([]byte, 64)
	RHat.X.PutBytesUnchecked(presig[:32])
	sHat.PutBytesUnchecked(presig[32:])

	// Step 16: Verify the signature
	// Return FAIL if EncVerify(bytes(Pp), ek, m, psig) fails
	if err := encVerify(presig, m, pBytes, t); err != nil {
		return nil, fmt.Errorf("verification of generated signature failed: %w", err)
	}

	return presig, nil
}

// encVerify implements the EncVerify algorithm as defined in the spec.
// It verifies that a pre-signature is valid with respect to the given
// public key, encryption key, and message.
func encVerify(
	psig []byte,
	m []byte,
	pubKeyBytes []byte,
	t *btcec.FieldVal,
) error {
	// Validate input lengths
	if len(psig) != 64 {
		return fmt.Errorf("wrong size for pre-signature (got %v, want 64)", len(psig))
	}
	if len(m) != chainhash.HashSize {
		return fmt.Errorf("wrong size for message (got %v, want %v)",
			len(m), chainhash.HashSize)
	}

	// Step 1: Let Pp = lift_x(int(pk)); return FAIL if that fails
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return err
	}
	// Fail if P is not a point on the curve
	if !pubKey.IsOnCurve() {
		return fmt.Errorf("pubkey point is not on curve")
	}

	// Step 2: Let Tp = lift_x(int(ek)); return FAIL if that fails
	Tp, err := liftX(t)
	if err != nil {
		return fmt.Errorf("failed to lift t: %w", err)
	}

	// Step 3: Let Rp = lift_x(int(psig[0:32])); return FAIL if that fails
	var rX btcec.FieldVal
	if overflow := rX.SetByteSlice(psig[0:32]); overflow {
		return fmt.Errorf("R x-coordinate exceeds field size")
	}
	Rp, err := liftX(&rX)
	if err != nil {
		return fmt.Errorf("failed to lift R x-coordinate: %w", err)
	}

	// Step 4: Let s = int(psig[32:64]); return FAIL if s >= n
	var s btcec.ModNScalar
	if overflow := s.SetByteSlice(psig[32:64]); overflow {
		return fmt.Errorf("s value exceeds curve order")
	}

	// Step 5: Compute RTp
	// If has_even_y(Rp + Tp), let RTp = Rp + Tp
	// Else if has_even_y(Rp - Tp), let RTp = Rp - Tp
	// Else return FAIL
	var RTp btcec.JacobianPoint

	// Try Rp + Tp first
	btcec.AddNonConst(Rp, Tp, &RTp)
	RTp.ToAffine()

	if RTp.Y.IsOdd() {
		// If Rp+Tp has odd y, try Rp-Tp
		btcec.AddNonConst(Rp, negatePoint(Tp), &RTp)
		RTp.ToAffine()

		if RTp.Y.IsOdd() {
			// If both Rp+Tp and Rp-Tp have odd y, this is an error
			return fmt.Errorf("both Rp+Tp and Rp-Tp have odd y")
		}
	}

	// Step 6: Compute challenge e
	// e = int(tagged_hash("BIP0340/challenge", bytes(RTp) || bytes(Pp) || m)) mod n
	var rtpBytes [chainhash.HashSize]byte
	RTp.X.PutBytesUnchecked(rtpBytes[:])
	commitment := chainhash.TaggedHash(
		chainhash.TagBIP0340Challenge, rtpBytes[:], pubKeyBytes, m,
	)
	var e btcec.ModNScalar
	e.SetBytes((*[ModNScalarSize]byte)(commitment))

	// Negate e to use AddNonConst for subtraction
	e.Negate()

	// Step 7: Compute Rrec = s*G - e*P
	var P, Rrec, sG, eP btcec.JacobianPoint
	pubKey.AsJacobian(&P)
	btcec.ScalarBaseMultNonConst(&s, &sG) // s*G
	btcec.ScalarMultNonConst(&e, &P, &eP) // -e*P
	btcec.AddNonConst(&sG, &eP, &Rrec)    // Rrec = s*G-e*P

	// Step 8: Return FAIL if Rrec is the point at infinity
	if (Rrec.X.IsZero() && Rrec.Y.IsZero()) || Rrec.Z.IsZero() {
		return fmt.Errorf("Rrec point is at infinity")
	}

	Rrec.ToAffine()

	// Step 9: Return FAIL if not has_even_y(Rrec)
	if Rrec.Y.IsOdd() {
		return fmt.Errorf("Rrec.y is odd")
	}

	// Step 10: Return FAIL if x(Rrec) != x(Rp)
	if !Rrec.X.Equals(&Rp.X) {
		return fmt.Errorf("x(Rrec) != x(Rp)")
	}

	// Step 11: Return SUCCESS only if no prior step failed
	return nil
}

// decrypt decrypts a pre-signature using a decryption key.
// This implements the Decrypt algorithm as defined in the spec.
func decrypt(psig []byte, dk []byte) ([]byte, error) {
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

	// Step 3-4: Let u' = int(dk)
	var u btcec.ModNScalar
	if overflow := u.SetByteSlice(dk); overflow {
		return nil, fmt.Errorf("decryption key exceeds curve order")
	}
	if u.IsZero() {
		return nil, fmt.Errorf("decryption key is zero")
	}

	// Step 5: Let T' = u' * G
	var T btcec.JacobianPoint
	btcec.ScalarBaseMultNonConst(&u, &T)
	T.ToAffine()

	// Step 6: Let (u, Tp) = (u', T') if has_even_y(T'), otherwise let (u, Tp) = (n - u', -T')
	var Tp btcec.JacobianPoint
	Tp = T
	if T.Y.IsOdd() {
		Tp.Y.Negate(1).Normalize()
		u.Negate()
	}

	// Step 7: Compute ss and RTp
	var RTp btcec.JacobianPoint
	var ss btcec.ModNScalar

	// Try Rp + Tp first
	btcec.AddNonConst(Rp, &Tp, &RTp)
	RTp.ToAffine()
	if !RTp.Y.IsOdd() {
		ss.Set(&s)
		ss.Add(&u)
	} else {
		// Try Rp - Tp
		btcec.AddNonConst(Rp, negatePoint(&Tp), &RTp)
		RTp.ToAffine()
		if !RTp.Y.IsOdd() {
			ss.Set(&s)
			u.Negate()
			ss.Add(&u)
		} else {
			return nil, fmt.Errorf("both Rp+Tp and Rp-Tp have odd y")
		}
	}

	// Step 8: Let sig = bytes(RTp) || bytes(ss)
	var sig [64]byte
	RTp.X.PutBytesUnchecked(sig[0:32])
	ss.PutBytesUnchecked(sig[32:64])

	// TODO: Step 9: Return FAIL if Verify(pk, m, sig) fails
	// skip verifying the signature for now

	// Step 10: Return sig
	return sig[:], nil
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

// unpackSchnorrSig extracts the r and s values from a Schnorr signature.
// It returns pointers to the r (FieldVal) and s (ModNScalar) components.
func unpackSchnorrSig(sig *schnorr.Signature) (*btcec.FieldVal, *btcec.ModNScalar) {
	sigBytes := sig.Serialize()
	var r btcec.FieldVal
	r.SetByteSlice(sigBytes[0:32])
	var s btcec.ModNScalar
	s.SetByteSlice(sigBytes[32:64])
	return &r, &s
}
