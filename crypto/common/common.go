package common

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/hkdf"
)

var (
	bSalt = [32]byte{
		0x3e, 0xcc, 0x4a, 0x35, 0xd9, 0x3a, 0x5a, 0x81,
		0xce, 0x46, 0xae, 0x70, 0xb8, 0x51, 0x9f, 0xd5,
		0x15, 0x14, 0x06, 0x3d, 0xef, 0x05, 0xdb, 0x9e,
		0xe4, 0xe8, 0x85, 0x65, 0x4f, 0xf7, 0x6d, 0x13,
	}

	zero32   = [32]byte{}
	zero4096 = [4096]byte{}
)

func ScalarBaseMultWithBlinding(k *btcec.ModNScalar) (*btcec.JacobianPoint, error) {
	var rInput [4096]byte
	if _, err := rand.Read(rInput[:]); err != nil {
		return nil, fmt.Errorf("failed to generate random input: %w", err)
	}
	secret := k.Bytes()

	// compute randomeness in defensive way
	rBytesExtract := hkdf.Extract(
		sha256.New,
		hkdf.Extract(sha256.New, secret[:], bSalt[:]),
		rInput[:],
	)

	// copy the first 32 bytes of the extracted bytes to rBytes and using it
	// as a blinding factor
	var rBytes [32]byte
	copy(rBytes[:], rBytesExtract[:32])

	// zero all intermediate values
	copy(secret[:], zero32[:])
	copy(rBytesExtract[:], zero32[:])
	copy(rInput[:], zero4096[:])

	// Generate a random blinding factor r
	r := new(btcec.ModNScalar)
	r.SetBytes(&rBytes)

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
