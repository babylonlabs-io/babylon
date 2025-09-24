package replay

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	abci "github.com/cometbft/cometbft/abci/types"

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

	driver.FinalizeCkptForEpoch(epoch1.EpochNumber)
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
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
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
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
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
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
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
	currEpochNumber := driver.GetEpoch().EpochNumber
	driver.ProgressTillFirstBlockTheNextEpoch()
	driver.FinalizeCkptForEpoch(currEpochNumber)

	msg := s1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
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

func TestPostRegistrationDelegation(t *testing.T) {
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
	driver.FinalizeCkptForEpoch(currnetEpochNunber)

	// Send post-registration delegation i.e first on BTC, then to Babylon
	msg := s1.CreateDelegationMessage(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
	)
	driver.ConfirmStakingTransactionOnBTC([]*bstypes.MsgCreateBTCDelegation{msg})
	require.NotNil(t, msg.StakingTxInclusionProof)
	s1.SendMessage(msg)
	driver.GenerateNewBlockAssertExecutionSuccess()
	// Activate through covenant signatures
	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	activeDelegations := driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)
}

func containsEvent(events []abci.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func attributeValueNonEmpty(event abci.Event, attributeKey string) bool {
	for _, attribute := range event.Attributes {
		if attribute.Key == attributeKey && len(attribute.Value) > 0 {
			return true
		}
	}
	return false
}

func TestAcceptSlashingTxAsUnbondingTx(t *testing.T) {
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

	var (
		stakingTime  = uint32(1000)
		stakingValue = int64(100000000)
	)
	s1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		stakingTime,
		stakingValue,
	)
	driver.GenerateNewBlockAssertExecutionSuccess()

	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	driver.ActivateVerifiedDelegations(1)
	activeDelegations := driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)

	prevStkTx, prevStkTxBz, err := bbn.NewBTCTxFromHex(activeDelegations[0].StakingTxHex)
	require.NoError(t, err)

	btcExpMsg := s1.CreateBtcStakeExpand(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		stakingTime,
		stakingValue,
		prevStkTx,
	)
	driver.GenerateNewBlockAssertExecutionSuccess()

	// A BTC delegation with stake expansion should be creatd
	// with pending state
	pendingDelegations := driver.GetPendingBTCDelegations(t)
	require.Len(t, pendingDelegations, 1)

	require.NotNil(t, pendingDelegations[0].StkExp)

	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	// After getting covenant sigs, stake expansion delegation
	// should be verified
	verifiedDels := driver.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDels, 1)
	require.NotNil(t, verifiedDels[0].StkExp)

	// wait stake expansion Tx be 'k' deep in BTC
	blockWithProofs, _ := driver.IncludeVerifiedStakingTxInBTC(1)
	require.Len(t, blockWithProofs.Proofs, 2)

	// Send MsgBTCUndelegate for the first delegation
	// to activate stake expansion
	spendingTx, err := bbn.NewBTCTxFromBytes(btcExpMsg.StakingTx)
	require.NoError(t, err)

	fundingTx, err := bbn.NewBTCTxFromBytes(btcExpMsg.FundingTx)
	require.NoError(t, err)

	params := driver.GetBTCStakingParams(t)
	spendingTxWithWitnessBz, slashingTxMsg := datagen.AddWitnessToStakeExpTx(
		t,
		prevStkTx.TxOut[0],
		fundingTx.TxOut[0],
		s1.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{fp1.BTCPrivateKey.PubKey()},
		uint16(stakingTime),
		stakingValue,
		spendingTx,
		driver.App.BTCLightClientKeeper.GetBTCNet(),
	)

	blockWithProofs, _ = driver.IncludeTxsInBTCAndConfirm([]*wire.MsgTx{slashingTxMsg})
	require.Len(t, blockWithProofs.Proofs, 2)

	msg := &bstypes.MsgBTCUndelegate{
		Signer:                        s1.AddressString(),
		StakingTxHash:                 prevStkTx.TxHash().String(),
		StakeSpendingTx:               spendingTxWithWitnessBz,
		StakeSpendingTxInclusionProof: btcstktypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[1]),
		FundingTransactions:           [][]byte{prevStkTxBz, btcExpMsg.FundingTx},
	}

	s1.SendMessage(msg)
	driver.GenerateNewBlockAssertExecutionSuccess()

	// After unbonding the initial delegation
	// the stake expansion should become active
	// and the initial delegation should become unbonded
	activeDelegations = driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)
	require.NotNil(t, activeDelegations[0].StkExp)

	unbondedDelegations := driver.GetUnbondedBTCDelegations(t)
	require.Len(t, unbondedDelegations, 1)
}
