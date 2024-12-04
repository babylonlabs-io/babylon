package types_test

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/btclightclient/types"
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

func TestRetargetBlockTestnet(t *testing.T) {
	btcNet := &chaincfg.SigNetParams
	blockActivationHeight := uint32(197535)
	for {

		if types.IsRetargetBlock(&types.BTCHeaderInfo{Height: blockActivationHeight}, btcNet) {
			t.Logf("Block height: %d is the first retarget block since 197535", blockActivationHeight)
			require.Equal(t, blockActivationHeight, uint32(195552))
			break
		}
		blockActivationHeight--
	}
}
