package common

import (
	"crypto/rand"

	"github.com/btcsuite/btcd/btcec/v2"
)

func ScalarBaseMultWithBlinding(k *btcec.ModNScalar) (*btcec.JacobianPoint, error) {
	// Generate a random blinding factor r
	r := new(btcec.ModNScalar)
	var rBytes [32]byte
	if _, err := rand.Read(rBytes[:]); err != nil {
		return nil, err
	}
	r.SetByteSlice(rBytes[:])

	// Compute (k+r)G
	kr := new(btcec.ModNScalar).Set(k).Add(r)
	var krG btcec.JacobianPoint
	btcec.ScalarBaseMultNonConst(kr, &krG)

	// Compute -rG
	rNeg := new(btcec.ModNScalar).Set(r).Negate()
	var rNegG btcec.JacobianPoint
	btcec.ScalarBaseMultNonConst(rNeg, &rNegG)

	// Convert to affine coordinates so that the addition is constant time
	krG.ToAffine()
	rNegG.ToAffine()

	// Add (k+r)G and -rG to get kG
	var R btcec.JacobianPoint
	btcec.AddNonConst(&krG, &rNegG, &R)

	return &R, nil
}
