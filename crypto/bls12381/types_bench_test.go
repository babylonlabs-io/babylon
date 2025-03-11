package bls12381

import (
	"crypto/rand"
	"testing"
)

func BenchmarkVerifyCompressed(b *testing.B) {
	// Generate random messages, keys and signatures
	msg := make([]byte, 32)
	rand.Read(msg)

	// Generate a random key pair
	sk, pk := GenKeyPair()

	// Create a signature
	sig := Sign(sk, msg)

	b.Run("WithSigGroupCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dummySig := new(BlsSig)
			dummySig.VerifyCompressed(sig, true, pk, false, msg, DST)
		}
	})

	b.Run("WithoutSigGroupCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dummySig := new(BlsSig)
			dummySig.VerifyCompressed(sig, false, pk, false, msg, DST)
		}
	})
}

func BenchmarkAggregateCompressed(b *testing.B) {
	// Generate random public keys
	size := 100
	pks := make([][]byte, 0, size)
	for i := 0; i < size; i++ {
		_, pk := GenKeyPair()
		pks = append(pks, pk.Bytes())
	}

	b.Run("WithGroupCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			aggPk := new(BlsMultiPubKey)
			aggPk.AggregateCompressed(pks, true) // With groupcheck
		}
	})

	b.Run("WithoutGroupCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			aggPk := new(BlsMultiPubKey)
			aggPk.AggregateCompressed(pks, false) // Without groupcheck
		}
	})
}
