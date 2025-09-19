package types_test

import (
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/types"
)

func FuzzSchnorrPubRand(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randBytes := datagen.GenRandomByteArray(r, 32)
		var fieldVal btcec.FieldVal
		fieldVal.SetByteSlice(randBytes)

		// FieldVal -> SchnorrPubRand -> FieldVal
		pubRand := types.NewSchnorrPubRandFromFieldVal(&fieldVal)
		fieldVal2 := pubRand.ToFieldValNormalized()
		require.True(t, fieldVal.Equals(fieldVal2))

		// SchnorrPubRand -> bytes -> SchnorrPubRand
		randBytes2 := pubRand.MustMarshal()
		pubRand2, err := types.NewSchnorrPubRand(randBytes)
		require.NoError(t, err)
		require.Equal(t, randBytes, randBytes2)
		require.Equal(t, pubRand, pubRand2)
	})
}
