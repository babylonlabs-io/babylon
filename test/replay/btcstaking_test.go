package replay

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/btcstaking"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	btcstakingtypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
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
		sdk.NewCoin("ubbn", sdkmath.NewInt(1)), // 1 ubbn
		stakingtypes.NewDescription("t", "", "", "", ""),
		stakingtypes.NewCommissionRates(
			sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(),
		),
		sdkmath.NewInt(1), // minSelfDelegation = 1
	)
	require.NoError(t, err)
	return msg
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

	prop := btcstakingtypes.MsgUpdateParams{
		Authority: appparams.AccGov.String(),
		Params:    p,
	}
	msgToSend := d.NewGovProp(&prop)
	d.SendTxWithMessagesSuccess(t, d.SenderInfo, DefaultGasLimit, defaultFeeCoin, msgToSend)

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

	fp1.RegisterFinalityProvider("")
	driver.GenerateNewBlockAssertExecutionSuccess()
	fp1.CommitRandomness()
	driver.GenerateNewBlockAssertExecutionSuccess()

	// Randomness timestamped
	currnetEpochNunber := driver.GetEpoch().EpochNumber
	driver.ProgressTillFirstBlockTheNextEpoch()
	driver.FinializeCkptForEpoch(currnetEpochNunber)

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

	fp1.RegisterFinalityProvider("")
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

	slashingTx, _, err := bbn.NewBTCTxFromHex(activeDelegations[0].SlashingTxHex)
	require.NoError(t, err)

	stakingTx, stakingTxBytes, err := bbn.NewBTCTxFromHex(activeDelegations[0].StakingTxHex)
	require.NoError(t, err)

	params := driver.GetBTCStakingParams(t)

	slashingTxbytes, slashingTxMsg := datagen.AddWitnessToSlashingTx(
		t,
		stakingTx.TxOut[0],
		s1.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{fp1.BTCPrivateKey.PubKey()},
		uint16(stakingTime),
		stakingValue,
		slashingTx,
		driver.App.BTCLightClientKeeper.GetBTCNet(),
	)

	blockWithProofs := driver.IncludeTxsInBTCAndConfirm([]*wire.MsgTx{slashingTxMsg})
	require.Len(t, blockWithProofs.Proofs, 2)

	msg := &bstypes.MsgBTCUndelegate{
		Signer:                        s1.AddressString(),
		StakingTxHash:                 stakingTx.TxHash().String(),
		StakeSpendingTx:               slashingTxbytes,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[1]),
		FundingTransactions:           [][]byte{stakingTxBytes},
	}
	s1.SendMessage(msg)
	txResults := driver.GenerateNewBlockAssertExecutionSuccessWithResults()
	// First result is injected checkpoint tx, second is the unbonding tx execution
	require.NotEmpty(t, txResults[1].Events)
	// Unbonding through slashing tx should be treated as EventBTCDelgationUnbondedEarly
	require.True(t, containsEvent(txResults[1].Events, "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"))

	unbondedDelegations := driver.GetUnbondedBTCDelegations(t)
	require.Len(t, unbondedDelegations, 1)
}

func TestSlashingFpWithManyMulistakedDelegations(t *testing.T) {
	tmpGas := DefaultGasLimit
	DefaultGasLimit = uint64(10_000_000)
	defer func() {
		DefaultGasLimit = tmpGas
	}()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	covSender := d.CreateCovenantSender()
	bbnFp := d.CreateNFinalityProviderAccounts(1)[0]
	numStakers := 50
	stakers := d.CreateNStakerAccounts(numStakers)
	d.GenerateNewBlockAssertExecutionSuccess()

	bbnFp.RegisterFinalityProvider("")
	d.GenerateNewBlockAssertExecutionSuccess()

	bbnFp.CommitRandomness()

	currentEpochNumber := d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinializeCkptForEpoch(currentEpochNumber)

	d.GenerateNewBlockAssertExecutionSuccess()

	stk2 := stakers[0]
	// send one btc delegation just to have voting power in the fp
	fps := []*bbn.BIP340PubKey{bbnFp.BTCPublicKey()}
	d.SendAndVerifyNDelegations(t, stk2, covSender, fps, 1)

	d.ActivateVerifiedDelegations(1)
	d.GenerateNewBlockAssertExecutionSuccess()

	// fp is activated
	activationHeight := d.GetActivationHeight(d.t)
	require.Greater(d.t, activationHeight, uint64(0))

	// create consumers
	const consumerID1 = "consumer1"
	const consumerID2 = "consumer2"
	ctx := d.Ctx().WithIsCheckTx(false)
	OpenChannelForConsumer(ctx, d.App, consumerID1)
	OpenChannelForConsumer(ctx, d.App, consumerID2)

	// 2. Register consumers
	consumer1 := d.RegisterConsumer(r, consumerID1)
	consumer2 := d.RegisterConsumer(r, consumerID2)
	require.NotNil(t, consumer1, consumer2)

	// 3. Create finality providers for each consumer
	fpsCons1 := d.CreateFinalityProviderForConsumer(consumer1)
	fpsCons2 := d.CreateFinalityProviderForConsumer(consumer2)
	require.NotNil(t, fpsCons1, fpsCons2)

	d.GenerateNewBlockAssertExecutionSuccess()
	fps = []*bbn.BIP340PubKey{bbnFp.BTCPublicKey(), fpsCons1.BTCPublicKey(), fpsCons2.BTCPublicKey()}

	d.MintNativeTo(t, covSender.Address(), 10000000_000000)

	// creates 200 btc delegations to slash it
	batchSize := 4
	totalActiveDels := len(d.GetActiveBTCDelegations(d.t))
	for _, stk := range stakers {
		d.MintNativeTo(t, stk.Address(), 10000000_000000)
		d.SendAndVerifyNDelegations(t, stk, covSender, fps, batchSize)

		d.GenerateNewBlockAssertExecutionSuccess()
		d.GenerateNewBlockAssertExecutionSuccess()

		verifiedDelegations := d.GetVerifiedBTCDelegations(t)
		require.Equal(t, len(verifiedDelegations), batchSize)

		d.ActivateVerifiedDelegations(batchSize)

		d.GenerateNewBlockAssertExecutionSuccess()
		activeDels := d.GetActiveBTCDelegations(d.t)

		totalActiveDels += batchSize
		require.Equal(t, len(activeDels), totalActiveDels)
	}
	// 200 dels + bbn del
	require.Equal(t, totalActiveDels, (batchSize*numStakers)+1)

	// unjails the fp so he can vote
	fpBtcPk := bbnFp.BTCPublicKey()
	fpBbn, err := d.App.BTCStakingKeeper.GetFinalityProvider(d.Ctx(), *fpBtcPk)
	require.NoError(t, err)

	fpBbn.Jailed = false
	d.App.BTCStakingKeeper.SetFinalityProvider(d.Ctx(), fpBbn)

	d.GenerateNewBlockAssertExecutionSuccess()
	bbnFp.CastVote(activationHeight)
	d.GenerateNewBlockAssertExecutionSuccess()

	// slash it
	bogusHash := datagen.GenRandomByteArray(r, 32)
	txRes := bbnFp.CastVoteForHash(activationHeight, bogusHash)
	require.LessOrEqual(t, txRes.GasUsed, int64(100_000))
	res := d.GenerateNewBlockAssertExecutionSuccessWithResults()
	require.NotEmpty(t, res)

	fpBbn, err = d.App.BTCStakingKeeper.GetFinalityProvider(d.Ctx(), *fpBtcPk)
	require.NoError(t, err)
	require.True(t, fpBbn.IsSlashed())
}
