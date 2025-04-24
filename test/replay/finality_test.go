package replay

import (
	"math/rand"
	"testing"
	"time"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/stretchr/testify/require"
)

func FuzzJailing(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 5)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		numFinalityProviders := datagen.RandomInRange(r, 2, 3)
		numDelPerFp := 2
		driverTempDir := t.TempDir()
		replayerTempDir := t.TempDir()
		driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
		driver.GenerateNewBlock()

		scenario := NewStandardScenario(driver)
		scenario.InitScenario(numFinalityProviders, numDelPerFp)

		// we do not have voting in this test, so wait until all fps are jailed
		driver.WaitTillAllFpsJailed(t)
		driver.GenerateNewBlock()
		activeFps := driver.GetActiveFpsAtCurrentHeight(t)
		require.Equal(t, 0, len(activeFps))

		// Replay all the blocks from driver and check appHash
		replayer := NewBlockReplayer(t, replayerTempDir)
		replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())
		// after replay we should have the same apphash
		require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
		require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
	})
}

func TestResumeFinalityOfSlashedFp(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlock()

	scn := NewStandardScenario(d)
	scn.InitScenario(2, 1) // 2 fps, 1 del each

	lastVotedBlkHeight, numBlocksFinalized := uint64(0), uint64(2)

	// finalize first 2 blocks, where both vote
	for blkHeight := scn.activationHeight; blkHeight < scn.activationHeight+numBlocksFinalized; blkHeight++ {
		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, false)

		for _, fp := range scn.finalityProviders {
			fp.CastVote(blkHeight)
		}

		d.GenerateNewBlockAssertExecutionSuccess()

		bl = d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, true)
		lastVotedBlkHeight = blkHeight
	}

	// one fp continues to vote, but the one to be jailed one does not vote
	jailedFp := scn.finalityProviders[1]
	lastFinalizedBlkHeight := lastVotedBlkHeight

	for {
		lastVotedBlkHeight++
		for _, fp := range scn.finalityProviders[:1] {
			fp.CastVote(lastVotedBlkHeight)
		}

		d.GenerateNewBlock()

		bl := d.GetIndexedBlock(lastVotedBlkHeight)
		require.Equal(t, bl.Finalized, false)

		fp := d.GetFp(*jailedFp.BTCPublicKey())
		if fp.Jailed {
			break
		}
	}

	// fp is slashed, sending bogus vote is not enough since the fp
	// is jailed, new votes are no accepted. It is needed to send a
	// selective slash with one of the BTC delegations stk txs
	jailedFp.SendSelectiveSlashingEvidence()
	d.GenerateNewBlock()

	slashedFp := d.GetFp(*jailedFp.BTCPublicKey())
	require.True(t, slashedFp.IsSlashed())

	// send gov proposal to resume finality
	prop := ftypes.MsgResumeFinalityProposal{
		Authority:     appparams.AccGov.String(),
		FpPksHex:      []string{slashedFp.BtcPk.MarshalHex()},
		HaltingHeight: uint32(lastFinalizedBlkHeight + 1), // fp voted in the last finalized
	}
	d.GovPropWaitPass(&prop)

	d.GenerateNewBlock()
	// check that the blocks got finalized
	for blkHeight := lastFinalizedBlkHeight + 1; blkHeight <= lastVotedBlkHeight; blkHeight++ {
		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, true)
	}

	// the fp in the voting power distribution cache should still be marked as slashed
	vpDstCache := d.GetVotingPowerDistCache(d.GetLastFinalizedBlock().Height)
	for _, vpFp := range vpDstCache.FinalityProviders {
		if vpFp.BtcPk.Equals(jailedFp.BTCPublicKey()) {
			require.True(d.t, vpFp.IsSlashed)
			require.Zero(d.t, vpFp.TotalBondedSat)
			continue
		}

		require.False(d.t, vpFp.IsJailed)
		require.False(d.t, vpFp.IsSlashed)
		require.NotZero(d.t, vpFp.TotalBondedSat)
	}

	// continue to be slashed status in btcstaking
	slashedFp = d.GetFp(*jailedFp.BTCPublicKey())
	require.True(t, slashedFp.IsSlashed())
}
