package replay

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/stretchr/testify/require"
)

// TestEpochFinalization checks whether we can finalize some epochs
func TestEpochFinalization(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(t, driverTempDir, replayerTempDir)
	// first finalize at least one block
	driver.GenerateNewBlock(t)
	epochingParams := driver.GetEpochingParams()

	epoch1 := driver.GetEpoch()
	require.Equal(t, epoch1.EpochNumber, uint64(1))

	for i := 0; i < int(epochingParams.EpochInterval); i++ {
		driver.GenerateNewBlock(t)
	}

	epoch2 := driver.GetEpoch()
	require.Equal(t, epoch2.EpochNumber, uint64(2))

	driver.FinializeCkptForEpoch(r, t, epoch1.EpochNumber)
}

func FuzzCreatingAndActivatingDelegations(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 3)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		numFinalityProviders := uint32(datagen.RandomInRange(r, 3, 7))
		numDelegationsPerFinalityProvider := uint32(datagen.RandomInRange(r, 20, 30))

		driverTempDir := t.TempDir()
		replayerTempDir := t.TempDir()
		driver := NewBabylonAppDriver(t, driverTempDir, replayerTempDir)
		// first finalize at least one block
		driver.GenerateNewBlock(t)
		stakingParams := driver.GetBTCStakingParams(t)

		fpInfos := GenerateNFinalityProviders(r, t, numFinalityProviders, driver.GetDriverAccountAddress())

		// register all finality providers
		for _, fpInfo := range fpInfos {
			driver.SendTxWithMsgsFromDriverAccount(t, fpInfo.MsgCreateFinalityProvider)
		}

		// register all delegations
		var allDelegationInfos []*datagen.CreateDelegationInfo
		for _, fpInfo := range fpInfos {
			delInfos := GenerateNBTCDelegationsForFinalityProvider(
				r,
				t,
				numDelegationsPerFinalityProvider,
				driver.GetDriverAccountAddress(),
				fpInfo,
				stakingParams,
			)
			allDelegationInfos = append(allDelegationInfos, delInfos...)
			msgs := ToCreateBTCDelegationMsgs(delInfos)
			driver.SendTxWithMsgsFromDriverAccount(t, msgs...)
		}

		allDelegations := driver.GetAllBTCDelegations(t)
		require.Equal(t, uint32(len(allDelegations)), numFinalityProviders*numDelegationsPerFinalityProvider)

		// add all covenant signatures
		for _, delInfo := range allDelegationInfos {
			driver.SendTxWithMsgsFromDriverAccount(t, MsgsToSdkMsg(delInfo.MsgAddCovenantSigs)...)
		}

		allVerifiedDelegations := driver.GetVerifiedBTCDelegations(t)
		require.Equal(t, uint32(len(allVerifiedDelegations)), numFinalityProviders*numDelegationsPerFinalityProvider)

		stakingTransactions := DelegationInfosToBTCTx(allDelegationInfos)
		blockWithProofs := driver.GenBlockWithTransactions(
			r,
			t,
			stakingTransactions,
		)
		// make staking txs k-deep
		driver.ExtendBTCLcWithNEmptyBlocks(r, t, 10)

		for i, stakingTx := range stakingTransactions {
			driver.SendTxWithMsgsFromDriverAccount(t, &bstypes.MsgAddBTCDelegationInclusionProof{
				Signer:                  driver.GetDriverAccountAddress().String(),
				StakingTxHash:           stakingTx.TxHash().String(),
				StakingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[i+1]),
			})
		}

		activeDelegations := driver.GetActiveBTCDelegations(t)
		require.Equal(t, uint32(len(activeDelegations)), numFinalityProviders*numDelegationsPerFinalityProvider)

		// Replay all the blocks from driver and check appHash
		replayer := NewBlockReplayer(t, replayerTempDir)
		replayer.ReplayBlocks(t, driver.FinalizedBlocks)
		// after replay we should have the same apphash
		require.Equal(t, driver.LastState.LastBlockHeight, replayer.LastState.LastBlockHeight)
		require.Equal(t, driver.LastState.AppHash, replayer.LastState.AppHash)
	})
}
