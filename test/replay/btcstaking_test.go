package replay

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	ibctmtypes "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
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
		info.RegisterFinalityProvider("")
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
	fp1.RegisterFinalityProvider("")
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

	fp1.RegisterFinalityProvider("")

	driver.GenerateNewBlockAssertExecutionSuccess()

	s1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
	)
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

	fp1.RegisterFinalityProvider("")
	driver.GenerateNewBlock()

	s1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
	)
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

	fp1.RegisterFinalityProvider("")
	driver.GenerateNewBlockAssertExecutionSuccess()

	s1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
	)
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

	fp1.RegisterFinalityProvider("")
	driver.GenerateNewBlockAssertExecutionSuccess()
	fp1.CommitRandomness()
	driver.GenerateNewBlockAssertExecutionSuccess()

	// Randomness timestamped
	currnetEpochNunber := driver.GetEpoch().EpochNumber
	driver.ProgressTillFirstBlockTheNextEpoch()
	driver.FinializeCkptForEpoch(currnetEpochNunber)

	s1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
	)
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
			[]*bbn.BIP340PubKey{scenario.finalityProviders[0].BTCPublicKey()},
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

func TestBadWrappedCreateValidator(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlock()

	poisonMsg := &types.MsgWrappedCreateValidator{
		MsgCreateValidator: MakeInnerMsg(t),
		Key:                nil, // triggers nil-pointer panic in VerifyPoP
	}

	resp, err := SendTxWithMessages(
		d.t,
		d.App,
		d.SenderInfo,
		poisonMsg,
	)
	require.Equal(t, resp.Code, uint32(1))
	require.Equal(t, resp.Log, "BLS key is nil")
	require.NoError(t, err)
}

func MakeInnerMsg(t *testing.T) *stakingtypes.MsgCreateValidator {
	priv := ed25519.GenPrivKey()
	valAddr := sdk.ValAddress(priv.PubKey().Address())
	consPub, _ := cryptocodec.FromCmtPubKeyInterface(priv.PubKey())

	msg, err := stakingtypes.NewMsgCreateValidator(
		valAddr.String(),
		consPub,
		sdk.NewCoin("ubbn", math.NewInt(1)), // 1 ubbn
		stakingtypes.NewDescription("t", "", "", "", ""),
		stakingtypes.NewCommissionRates(
			math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec(),
		),
		math.NewInt(1), // minSelfDelegation = 1
	)
	require.NoError(t, err)
	return msg
}

func TestMultiConsumerDelegation(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	// 1. Set up mock IBC clients for each consumer
	ctx := driver.App.BaseApp.NewContext(false)
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, "consumer1", &ibctmtypes.ClientState{})
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, "consumer2", &ibctmtypes.ClientState{})
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, "consumer3", &ibctmtypes.ClientState{})
	driver.GenerateNewBlock()

	// 2. Register consumers with different max_multi_staked_fps limits
	consumer1 := driver.RegisterConsumer("consumer1", 2)
	consumer2 := driver.RegisterConsumer("consumer2", 3)
	consumer3 := driver.RegisterConsumer("consumer3", 4)
	// Create a Babylon FP (registered without consumer ID)
	babylonFp := driver.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	// 3. Create finality providers for each consumer
	fp1 := driver.CreateFinalityProviderForConsumer(consumer1)
	fp2 := driver.CreateFinalityProviderForConsumer(consumer2)
	fp3 := driver.CreateFinalityProviderForConsumer(consumer3)
	// Generate blocks to process registrations
	driver.GenerateNewBlockAssertExecutionSuccess()
	staker := driver.CreateNStakerAccounts(1)[0]

	// 4. Create a delegation with three consumer FPs and one Babylon FP - should fail because total FPs (4) > min(max_multi_staked_fps) which is 2
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey(), fp2.BTCPublicKey(), fp3.BTCPublicKey(), babylonFp.BTCPublicKey()},
		1000,
		100000000,
	)
	driver.GenerateNewBlockAssertExecutionFailure()

	// 5. Create a valid delegation with 2 FPs (including Babylon FP)
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{babylonFp.BTCPublicKey(), fp1.BTCPublicKey()},
		1000,
		100000000,
	)
	driver.GenerateNewBlockAssertExecutionSuccess()

	// 6. Replay all blocks and verify state
	replayer := NewBlockReplayer(t, replayerTempDir)

	// Set up IBC client states in the replayer before replaying blocks
	replayerCtx := replayer.App.BaseApp.NewContext(false)
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, "consumer1", &ibctmtypes.ClientState{})
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, "consumer2", &ibctmtypes.ClientState{})
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, "consumer3", &ibctmtypes.ClientState{})

	// Replay all the blocks from driver and check appHash
	replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())
	// After replay we should have the same apphash and last block height
	require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
	require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
}
