package keeper_test

import (
	"fmt"
	"math/rand"
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

func benchmarkTallyBlocks(b *testing.B, numFPs int) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	fKeeper, ctx := keepertest.FinalityKeeper(b, bsKeeper, iKeeper, cKeeper)

	// activate BTC staking protocol at a random height
	activatedHeight := uint64(1)
	// add mock queries to GetBTCStakingActivatedHeight
	ctx = datagen.WithCtxHeight(ctx, uint64(activatedHeight))
	bsKeeper.EXPECT().GetBTCStakingActivatedHeight(gomock.Any()).Return(activatedHeight, nil).AnyTimes()

	// simulate fp set
	fpSet := map[string]uint64{}
	for i := 0; i < numFPs; i++ {
		votedFpPK, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(b, err)
		fpSet[votedFpPK.MarshalHex()] = 1
	}
	bsKeeper.EXPECT().GetVotingPowerTable(gomock.Any(), gomock.Any()).Return(fpSet).AnyTimes()

	// TODO: test incentive
	bsKeeper.EXPECT().GetVotingPowerDistCache(gomock.Any(), gomock.Any()).Return(bstypes.NewVotingPowerDistCache(), nil).AnyTimes()
	iKeeper.EXPECT().RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	bsKeeper.EXPECT().RemoveVotingPowerDistCache(gomock.Any(), gomock.Any()).Return().AnyTimes()
	// Start the CPU profiler
	cpuProfileFile := fmt.Sprintf("/tmp/finality-tally-blocks-%d-cpu.pprof", numFPs)
	f, err := os.Create(cpuProfileFile)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		b.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	// Reset timer before the benchmark loop starts
	b.ResetTimer()

	// tally a block
	for i := int(activatedHeight); i < b.N; i++ {
		b.StopTimer()

		height := uint64(i)
		// increment height in ctx
		ctx = datagen.WithCtxHeight(ctx, height)
		// index blocks
		fKeeper.SetBlock(ctx, &types.IndexedBlock{
			Height:    height,
			AppHash:   datagen.GenRandomByteArray(r, 32),
			Finalized: false,
		})
		// give votes to the block
		for fpPKHex := range fpSet {
			votedFpPK, err := bbn.NewBIP340PubKeyFromHex(fpPKHex)
			require.NoError(b, err)
			votedSig, err := bbn.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
			require.NoError(b, err)
			fKeeper.SetSig(ctx, height, votedFpPK, votedSig)
		}

		b.StartTimer()

		fKeeper.TallyBlocks(ctx)
	}
}

func BenchmarkTallyBlocks_10(b *testing.B)  { benchmarkTallyBlocks(b, 10) }
func BenchmarkTallyBlocks_50(b *testing.B)  { benchmarkTallyBlocks(b, 50) }
func BenchmarkTallyBlocks_100(b *testing.B) { benchmarkTallyBlocks(b, 100) }
