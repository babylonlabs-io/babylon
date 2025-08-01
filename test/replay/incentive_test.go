package replay

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	minttypes "github.com/babylonlabs-io/babylon/v3/x/mint/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
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

func TestAddBsnRewardsMathOverflow(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)

	d.GenerateNewBlock()

	covSender := d.CreateCovenantSender()
	require.NotNil(t, covSender)

	consumerID := "bsn-consumer-0"
	d.App.IBCKeeper.ClientKeeper.SetClientState(d.Ctx(), consumerID, &ibctmtypes.ClientState{})
	d.GenerateNewBlock()

	consumer0 := d.RegisterConsumer(r, consumerID)
	d.GenerateNewBlockAssertExecutionSuccess()

	babylonFp := d.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	consFps := []*FinalityProvider{
		d.CreateFinalityProviderForConsumer(consumer0),
		d.CreateFinalityProviderForConsumer(consumer0),
	}

	staker := d.CreateNStakerAccounts(1)[0]
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{consFps[0].BTCPublicKey(), babylonFp.BTCPublicKey()},
		1000,
		100000000,
	)
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{consFps[1].BTCPublicKey(), babylonFp.BTCPublicKey()},
		1000,
		200000000,
	)

	d.GenerateNewBlockAssertExecutionSuccess()

	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	d.ActivateVerifiedDelegations(2)
	activeDelegations := d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 2)

	// send all the rewards to the same FP to force the math overflow
	fpRatios := []types.FpRatio{
		{BtcPk: consFps[0].BTCPublicKey(), Ratio: math.LegacyOneDec()},
	}

	bsnRewardCoinsMaxSupply := datagen.GenRandomCoinsMaxSupply(r)
	err := d.App.MintKeeper.MintCoins(d.Ctx(), bsnRewardCoinsMaxSupply)
	require.NoError(t, err)

	recipient := d.GetDriverAccountAddress()
	err = d.App.BankKeeper.SendCoinsFromModuleToAccount(d.Ctx(), minttypes.ModuleName, recipient, bsnRewardCoinsMaxSupply)
	require.NoError(t, err)
	d.GenerateNewBlockAssertExecutionSuccess()

	// FP current rewards now have the decimals, the error of int overflow should be thrown
	// when the user send the message.
	d.AddBsnRewardsFromDriver(consumer0.ID, bsnRewardCoinsMaxSupply, fpRatios)
	txResults := d.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, txResults, 1)
	require.Equal(t, uint32(1133), txResults[0].Code)
	require.Contains(t, txResults[0].Log, "integer overflow")
}
