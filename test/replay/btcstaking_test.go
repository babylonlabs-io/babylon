package replay

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
)

// TestEpochFinalization checks whether we can finalize some epochs
func TestEpochFinalization(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	// first finalize at least one block
	driver.GenerateNewBlock()
	epochingParams := driver.GetEpochingParams()

	epoch1 := driver.GetEpoch()
	require.Equal(t, epoch1.EpochNumber, uint64(1))

	for i := 0; i < int(epochingParams.EpochInterval); i++ {
		driver.GenerateNewBlock()
	}

	epoch2 := driver.GetEpoch()
	require.Equal(t, epoch2.EpochNumber, uint64(2))

	driver.FinializeCkptForEpoch(epoch1.EpochNumber)
}

func FuzzCreatingAndActivatingDelegations(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 3)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		numFinalityProviders := datagen.RandomInRange(r, 2, 3)
		numDelegationsPerFinalityProvider := datagen.RandomInRange(r, 1, 2)
		driverTempDir := t.TempDir()
		replayerTempDir := t.TempDir()
		driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
		// first finalize at least one block
		driver.GenerateNewBlock()

		scenario := NewStandardScenario(driver)
		scenario.InitScenario(numFinalityProviders, numDelegationsPerFinalityProvider)

		// Replay all the blocks from driver and check appHash
		replayer := NewBlockReplayer(t, replayerTempDir)
		replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())
		// after replay we should have the same apphash
		require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
		require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
	})
}

func TestNewAccountCreation(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	// first finalize at least one block
	driver.GenerateNewBlock()

	stakers := driver.CreateNStakerAccounts(5)
	require.Len(t, stakers, 5)

	fps := driver.CreateNFinalityProviderAccounts(3)
	require.Len(t, fps, 3)
}

func TestFinalityProviderRegistration(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()
	numFp := 3
	infos := driver.CreateNFinalityProviderAccounts(numFp)

	for _, info := range infos {
		info.RegisterFinalityProvider()
	}

	driver.GenerateNewBlock()

	// Check all fps registered themselves
	allFp := driver.GetAllFps(t)
	require.Len(t, allFp, numFp)
}

func TestFinalityProviderCommitRandomness(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]

	// register and commit in one block
	fp1.RegisterFinalityProvider()
	fp1.CommitRandomness()

	driver.GenerateNewBlock()
}

func TestSendingDelegation(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]

	fp1.RegisterFinalityProvider()

	driver.GenerateNewBlockAssertExecutionSuccess()

	msg := s1.CreatePreApprovalDelegation(
		fp1.BTCPublicKey(),
		1000,
		100000000,
	)
	require.NotNil(t, msg)
	driver.GenerateNewBlockAssertExecutionSuccess()

	delegations := driver.GetAllBTCDelegations(t)
	require.Len(t, delegations, 1)
}

func TestSendingCovenantSignatures(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	covSender := driver.CreateCovenantSender()
	require.NotNil(t, covSender)

	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]

	fp1.RegisterFinalityProvider()
	driver.GenerateNewBlock()

	msg := s1.CreatePreApprovalDelegation(
		fp1.BTCPublicKey(),
		1000,
		100000000,
	)
	require.NotNil(t, msg)
	driver.GenerateNewBlockAssertExecutionSuccess()

	pendingDels := driver.GetPendingBTCDelegations(t)
	require.Len(t, pendingDels, 1)

	covSender.SendCovenantSignatures()
	driver.GenerateNewBlock()

	verifiedDels := driver.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDels, 1)
}

func TestActivatingDelegation(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	covSender := driver.CreateCovenantSender()
	require.NotNil(t, covSender)

	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]

	fp1.RegisterFinalityProvider()
	driver.GenerateNewBlockAssertExecutionSuccess()

	msg := s1.CreatePreApprovalDelegation(
		fp1.BTCPublicKey(),
		1000,
		100000000,
	)
	require.NotNil(t, msg)

	driver.GenerateNewBlockAssertExecutionSuccess()

	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	driver.ActivateVerifiedDelegations(1)
	activeDelegations := driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)
}

func TestVoting(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	covSender := driver.CreateCovenantSender()
	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]
	require.NotNil(t, s1)

	fp1.RegisterFinalityProvider()
	driver.GenerateNewBlockAssertExecutionSuccess()
	fp1.CommitRandomness()
	driver.GenerateNewBlockAssertExecutionSuccess()

	// Randomness timestamped
	currnetEpochNunber := driver.GetEpoch().EpochNumber
	driver.ProgressTillFirstBlockTheNextEpoch()
	driver.FinializeCkptForEpoch(currnetEpochNunber)

	msg := s1.CreatePreApprovalDelegation(
		fp1.BTCPublicKey(),
		1000,
		100000000,
	)
	require.NotNil(t, msg)

	driver.GenerateNewBlockAssertExecutionSuccess()
	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	driver.ActivateVerifiedDelegations(1)
	activeDelegations := driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)
	// need to generate new block to get the activation height
	driver.GenerateNewBlock()

	activationHeight := driver.GetActivationHeight(t)
	require.Greater(t, activationHeight, uint64(0))
	activeFps := driver.GetActiveFpsAtHeight(t, activationHeight)
	require.Len(t, activeFps, 1)

	fp1.CastVote(activationHeight)
	driver.GenerateNewBlock()
	block := driver.GetIndexedBlock(activationHeight)
	require.NotNil(t, block)
	require.True(t, block.Finalized)
}

func TestStakingAndFinalizingBlocks(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	scenario := NewStandardScenario(driver)
	scenario.InitScenario(4, 1)

	// BTC finalize 3 blocks
	for i := scenario.activationHeight; i < scenario.activationHeight+3; i++ {
		bl := driver.GetIndexedBlock(i)
		require.NotNil(t, bl)
		require.Equal(t, bl.Finalized, false)

		for _, fp := range scenario.finalityProviders {
			fp.CastVote(i)
		}

		driver.GenerateNewBlockAssertExecutionSuccess()

		bl = driver.GetIndexedBlock(i)
		require.NotNil(t, bl)
		require.Equal(t, bl.Finalized, true)
	}
}

func TestStakingAndFinalizingMultipleBlocksAtOnce(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driver := NewBabylonAppDriverTmpDir(r, t)
	driver.GenerateNewBlock()

	scenario := NewStandardScenario(driver)
	scenario.InitScenario(4, 1)

	numBlocksInTest := uint64(10)

	// cast votes of the first 2 fps on 10 blocks, block should not be finalized
	for i := scenario.activationHeight; i < scenario.activationHeight+numBlocksInTest; i++ {
		bl := driver.GetIndexedBlock(i)
		require.NotNil(t, bl)
		require.Equal(t, bl.Finalized, false)

		for _, fp := range scenario.finalityProviders[:2] {
			fp.CastVote(i)
		}

		driver.GenerateNewBlockAssertExecutionSuccess()

		bl = driver.GetIndexedBlock(i)
		require.NotNil(t, bl)
		require.Equal(t, bl.Finalized, false)
	}

	// FP[3] votes for every block except the activation one.Block should not be finalized
	// as activtion block is not finalized
	for i := scenario.activationHeight + 1; i < scenario.activationHeight+numBlocksInTest; i++ {
		scenario.finalityProviders[3].CastVote(i)
		bl := driver.GetIndexedBlock(i)
		require.NotNil(t, bl)
		require.Equal(t, bl.Finalized, false)
	}

	// FP[3] votes for the activation block, all previous blocks should be finalized
	scenario.finalityProviders[3].CastVote(scenario.activationHeight)
	driver.GenerateNewBlock()

	for i := scenario.activationHeight; i < scenario.activationHeight+numBlocksInTest; i++ {
		bl := driver.GetIndexedBlock(i)
		require.NotNil(t, bl)
		require.Equal(t, bl.Finalized, true)
	}
}

func TestSlashingandFinalizingBlocks(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driver := NewBabylonAppDriverTmpDir(r, t)
	driver.GenerateNewBlock()

	scenario := NewStandardScenario(driver)
	scenario.InitScenario(6, 1) // 6 finality providers

	numBlocksInTest := uint64(10)
	lastVotedBlkHeight := scenario.FinalityFinalizeBlocks(scenario.activationHeight, numBlocksInTest)

	indexSlashFp1 := 1
	indexSlashFp2 := 2
	jailedFp1 := scenario.finalityProviders[indexSlashFp1]
	jailedFp2 := scenario.finalityProviders[indexSlashFp2]

	bl := driver.GetIndexedBlock(lastVotedBlkHeight)
	require.Equal(t, bl.Finalized, true)

	// 2 fps not voting
	for i := uint64(0); i < numBlocksInTest; i++ {
		lastVotedBlkHeight++
		for i, fp := range scenario.finalityProviders {
			if i != indexSlashFp1 {
				fp.CastVote(lastVotedBlkHeight)
			}
		}

		driver.GenerateNewBlock()

		bl := driver.GetIndexedBlock(lastVotedBlkHeight)
		require.Equal(t, bl.Finalized, true)
	}

	scenario.finalityProviders[indexSlashFp1].SendSelectiveSlashingEvidence()
	scenario.finalityProviders[indexSlashFp2].SendSelectiveSlashingEvidence()

	driver.GenerateNewBlock()

	slashedFp1 := driver.GetFp(*jailedFp1.BTCPublicKey())
	require.True(t, slashedFp1.IsSlashed())
	slashedFp2 := driver.GetFp(*jailedFp2.BTCPublicKey())
	require.True(t, slashedFp2.IsSlashed())

	driver.GenerateNewBlock()

	bl = driver.GetIndexedBlock(lastVotedBlkHeight)
	require.Equal(t, bl.Finalized, true)

	require.Len(t, driver.GetActiveFpsAtCurrentHeight(t), 4)
}

func TestActivatingDelegationOnSlashedFp(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driver := NewBabylonAppDriverTmpDir(r, t)
	driver.GenerateNewBlock()

	scenario := NewStandardScenario(driver)
	scenario.InitScenario(6, 1) // 6 finality providers

	covSender := driver.CreateCovenantSender()
	require.NotNil(t, covSender)

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]

	numBlocksInTest := uint64(10)
	lastVotedBlkHeight := scenario.FinalityFinalizeBlocks(scenario.activationHeight, numBlocksInTest)

	indexSlashFp1 := 1
	indexSlashFp2 := 2
	jailedFp1 := scenario.finalityProviders[indexSlashFp1]
	jailedFp2 := scenario.finalityProviders[indexSlashFp2]

	bl := driver.GetIndexedBlock(lastVotedBlkHeight)
	require.Equal(t, bl.Finalized, true)

	// 2 fps not voting
	for i := uint64(0); i < numBlocksInTest; i++ {
		lastVotedBlkHeight++
		for i, fp := range scenario.finalityProviders {
			if i != indexSlashFp1 {
				fp.CastVote(lastVotedBlkHeight)
			}
		}

		driver.GenerateNewBlock()

		bl := driver.GetIndexedBlock(lastVotedBlkHeight)
		require.Equal(t, bl.Finalized, true)
	}

	scenario.finalityProviders[indexSlashFp1].SendSelectiveSlashingEvidence()
	scenario.finalityProviders[indexSlashFp2].SendSelectiveSlashingEvidence()

	driver.GenerateNewBlock()

	slashedFp1 := driver.GetFp(*jailedFp1.BTCPublicKey())
	require.True(t, slashedFp1.IsSlashed())
	slashedFp2 := driver.GetFp(*jailedFp2.BTCPublicKey())
	require.True(t, slashedFp2.IsSlashed())

	driver.GenerateNewBlock()

	bl = driver.GetIndexedBlock(lastVotedBlkHeight)
	require.Equal(t, bl.Finalized, true)

	require.Len(t, driver.GetActiveFpsAtCurrentHeight(t), 4)

	msg := s1.CreatePreApprovalDelegation(
		jailedFp1.BTCPublicKey(),
		1000,
		100000000,
	)
	require.NotNil(t, msg)

	driver.GenerateNewBlock()

	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	verifiedDelegations := driver.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDelegations, 0)

	bl = driver.GetIndexedBlock(lastVotedBlkHeight)
	require.Equal(t, bl.Finalized, true)
}

func TestJailingFinalityProvider(t *testing.T) {
    t.Parallel()
    r := rand.New(rand.NewSource(time.Now().UnixNano()))
    driver := NewBabylonAppDriverTmpDir(r, t)
    driver.GenerateNewBlock()

    scenario := NewStandardScenario(driver)
    scenario.InitScenario(2, 1)

    fp := scenario.finalityProviders[0]

    for i := 0; i < 10; i++ {
        driver.GenerateNewBlock()
    }

    jailedFp := driver.GetFp(*fp.BTCPublicKey())
    require.True(t, jailedFp.Jailed, "FP should be jailed after missing votes")
}