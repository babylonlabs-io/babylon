package replay

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
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
	lastVotedBlkHeight := scenario.FinalityFinalizeBlocks(scenario.activationHeight, numBlocksInTest, scenario.FpMapBtcPkHex())

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
	scenario.InitScenario(4, 1)

	for j := 0; j < 1; j++ {
		scenario.stakers[0].CreatePreApprovalDelegation(
			scenario.finalityProviders[0].BTCPublicKey(),
			1000,
			100000000,
		)
	}

	scenario.driver.GenerateNewBlockAssertExecutionSuccess()
	pendingDelegations := scenario.driver.GetPendingBTCDelegations(scenario.driver.t)
	require.Equal(scenario.driver.t, 1, len(pendingDelegations))

	scenario.covenant.SendCovenantSignatures()
	scenario.driver.GenerateNewBlockAssertExecutionSuccess()

	numBlocksInTest := uint64(10)
	lastVotedBlkHeight := scenario.FinalityFinalizeBlocks(scenario.activationHeight, numBlocksInTest, scenario.FpMapBtcPkHex())

	for i := uint64(0); i < numBlocksInTest; i++ {
		lastVotedBlkHeight++
		for i, fp := range scenario.finalityProviders {
			if i != 0 {
				fp.CastVote(lastVotedBlkHeight)
			}
		}

		driver.GenerateNewBlock()

		bl := driver.GetIndexedBlock(lastVotedBlkHeight)
		require.Equal(t, bl.Finalized, true)
	}

	scenario.finalityProviders[0].SendSelectiveSlashingEvidence()

	driver.GenerateNewBlock()

	slashedFp1 := driver.GetFp(*scenario.finalityProviders[0].BTCPublicKey())
	require.True(t, slashedFp1.IsSlashed())

	verifiedDelegations := scenario.driver.GetVerifiedBTCDelegations(scenario.driver.t)
	require.Equal(scenario.driver.t, len(verifiedDelegations), 1)

	scenario.driver.ActivateVerifiedDelegations(1)
	scenario.driver.GenerateNewBlockAssertExecutionSuccess()

	activationHeight := scenario.driver.GetActivationHeight(scenario.driver.t)
	require.Greater(scenario.driver.t, activationHeight, uint64(0))

	activeFps := scenario.driver.GetActiveFpsAtHeight(scenario.driver.t, activationHeight)
	require.Equal(scenario.driver.t, len(activeFps), 4)

	require.True(t, slashedFp1.IsSlashed())

	for i := uint64(0); i < numBlocksInTest; i++ {
		lastVotedBlkHeight++
		for _, fp := range scenario.finalityProviders {
			fp.CastVote(lastVotedBlkHeight)
		}

		resp := driver.GenerateNewBlock()
		require.NotNil(t, resp)

		var containsSlashing bool = false

		for _, res := range resp.TxResults {
			if res.Code == 1104 {
				containsSlashing = true
			}
		}
		require.True(t, containsSlashing)
	}
}

func TestJailingFinalityProvider(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driver := NewBabylonAppDriverTmpDir(r, t)
	driver.GenerateNewBlock()

	numBlocksFinalized := uint64(2)
	scenario := NewStandardScenario(driver)
	scenario.InitScenario(2, 1)

	lastVotedBlkHeight := scenario.FinalityFinalizeBlocks(scenario.activationHeight, numBlocksFinalized, scenario.FpMapBtcPkHex())

	fp := scenario.finalityProviders[0]

	for {
		lastVotedBlkHeight++
		for i, fp := range scenario.finalityProviders {
			if i != 0 {
				fp.CastVote(lastVotedBlkHeight)
			}
		}

		driver.GenerateNewBlock()

		bl := driver.GetIndexedBlock(lastVotedBlkHeight)
		require.Equal(t, bl.Finalized, false)

		fp := driver.GetFp(*fp.BTCPublicKey())
		if fp.Jailed {
			break
		}
	}

	fp.SendSelectiveSlashingEvidence()
	driver.GenerateNewBlock()

	activeFps := driver.GetActiveFpsAtCurrentHeight(t)
	require.Equal(t, 1, len(activeFps))
}

func TestBadUnbondingFeeParams(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlock()

	numBlocksFinalized := uint64(2)
	scn := NewStandardScenario(d)
	scn.InitScenario(2, 1)

	scn.FinalityFinalizeBlocksAllVotes(scn.activationHeight, numBlocksFinalized)

	d.GenerateNewBlockAssertExecutionSuccess()

	btcStkK := d.App.BTCStakingKeeper
	p := btcStkK.GetParams(d.Ctx())

	// bad param creation
	p.BtcActivationHeight += 10
	p.MinStakingValueSat = 100000
	p.UnbondingFeeSat = -1
	p.SlashingRate = sdkmath.LegacyNewDecWithPrec(1, 1)

	prop := btcstktypes.MsgUpdateParams{
		Authority: appparams.AccGov.String(),
		Params:    p,
	}
	msgToSend := d.NewGovProp(&prop)
	d.SendTxWithMessagesSuccess(t, d.SenderInfo, defaultGasLimit, defaultFeeCoin, msgToSend)

	txResults := d.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, txResults, 1)
	require.Equal(t, uint32(12), txResults[0].Code)
	require.Contains(t, txResults[0].Log, btcstaking.ErrInvalidUnbondingFee.Error())
}
