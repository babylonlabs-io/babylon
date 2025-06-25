package types_test

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"
)

func FuzzCumulativeWork(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		numa := r.Uint64()
		numb := r.Uint64()
		biga := sdkmath.NewUint(numa)
		bigb := sdkmath.NewUint(numb)

		gotSum := types.CumulativeWork(biga, bigb)

		expectedSum := sdkmath.NewUint(0)
		expectedSum = expectedSum.Add(biga)
		expectedSum = expectedSum.Add(bigb)

		if !expectedSum.Equal(gotSum) {
			t.Errorf("Cumulative work does not correspond to actual one")
		}
	})
}

func TestRetargetBlock(t *testing.T) {
	expectedBaseBlockHeightTestnet4 := uint32(195552)
	require.True(t, types.IsRetargetBlock(&types.BTCHeaderInfo{Height: expectedBaseBlockHeightTestnet4}, &chaincfg.SigNetParams))

	baseBtcHeaderMainnetHeight := uint32(854784)
	cap1ActivationHeight := uint32(857910)
	require.True(t, types.IsRetargetBlock(&types.BTCHeaderInfo{Height: baseBtcHeaderMainnetHeight}, &chaincfg.MainNetParams))
	require.True(t, baseBtcHeaderMainnetHeight < cap1ActivationHeight)
}
