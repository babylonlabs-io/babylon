package replay

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestBtcRewardTrackerAtRewardedBabylonBlockAndNotLatestState(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	_, finalityK := d.App.BTCStakingKeeper, d.App.FinalityKeeper
	ictvK := d.App.IncentiveKeeper

	scn := NewStandardScenario(d)
	// 3 fps each with one delegation
	scn.InitScenario(3, 1)

	d.GenerateBlocksUntilHeight(scn.activationHeight)
	blksToFinalize := uint64(10)

	// finalize 10 blocks and check rewards
	lastFinalizedHeight := scn.FinalityFinalizeBlocksAllVotes(scn.activationHeight, blksToFinalize)
	// makes sure that all the events are processed and the 10 finalized blocks are rewarded.
	d.GenerateBlocksUntilLastProcessedBtcStkEventsHeightIs(lastFinalizedHeight)

	rewardsByStakerFor10Blocks := scn.IctvWithdrawBtcStakerRewardsByAddr()
	require.True(t, AllCoinsEqual(rewardsByStakerFor10Blocks))

	lastProcessedBtcStkEvtsHeight, err := ictvK.GetRewardTrackerEventLastProcessedHeight(d.Ctx())
	require.NoError(t, err)
	require.EqualValues(t, lastFinalizedHeight, lastProcessedBtcStkEvtsHeight)

	currBlock := uint64(d.Ctx().BlockHeader().Height)
	diffBlockAndLastFinalized := currBlock - lastFinalizedHeight
	// there is 5 blk ahead of finalization, finality sig time out is 4
	require.EqualValues(t, diffBlockAndLastFinalized, 5)

	d.GenerateNewBlockAssertExecutionSuccess()

	lastVpDstCache := finalityK.GetVotingPowerDistCache(d.Ctx(), currBlock-1)
	// A new very big BTC delegation is made, but 5 bbn blocks already passed
	bigFp := scn.finalityProviders[datagen.RandomInt(r, len(scn.finalityProviders))]
	bigDel := scn.stakers[datagen.RandomInt(r, len(scn.stakers))]
	scn.CreateActiveBtcDel(bigFp, bigDel, int64(lastVpDstCache.TotalVotingPower*2))

	// cast all votes for the diff blocks
	mapFps := scn.FpMapBtcPkHex()
	startVoteHeight := lastFinalizedHeight + 1
	lastVoteHeight := currBlock
	for blkToVote := startVoteHeight; blkToVote < lastVoteHeight; blkToVote++ {
		scn.FinalityCastVotes(blkToVote, mapFps)
		lastFinalizedHeight++
	}

	// finalize the block
	d.GenerateNewBlockAssertExecutionSuccess()

	// produce blocks until all the events are processed
	d.GenerateBlocksUntilLastProcessedBtcStkEventsHeightIs(lastFinalizedHeight)

	rewardsByStakerForDiffBlocks := scn.IctvWithdrawBtcStakerRewardsByAddr()
	require.Len(t, rewardsByStakerFor10Blocks, 3)

	// all the values in the rewards should match as the new big btc delegation should not count
	require.True(t, AllCoinsEqual(rewardsByStakerForDiffBlocks))

	// finalize a few more blocks to check that the big delegation did modified the weights later on
	scn.FinalityFinalizeBlocksAllVotes(lastFinalizedHeight+1, 8)

	lastRwdCheck := scn.IctvWithdrawBtcStakerRewardsByAddr()
	require.False(t, AllCoinsEqual(lastRwdCheck))
}

func AllCoinsEqual(coins map[string]sdk.Coin) bool {
	var ref *sdk.Coin

	for _, coin := range coins {
		if ref == nil {
			ref = &coin
			continue
		}
		if !coin.IsEqual(*ref) {
			return false
		}
	}

	return true
}
