package e2e

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"
	feegrantcli "cosmossdk.io/x/feegrant/client/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/crypto/eots"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/x/incentive/types"
)

var (
	r   = rand.New(rand.NewSource(time.Now().Unix()))
	net = &chaincfg.SimNetParams
	// finality provider
	fpBTCSK, _, _ = datagen.GenRandomBTCKeyPair(r)
	cacheFP       *bstypes.FinalityProvider
	// BTC delegation
	delBTCSK, delBTCPK, _ = datagen.GenRandomBTCKeyPair(r)
	// covenant
	covenantSKs, _, covenantQuorum = bstypes.DefaultCovenantCommittee()

	stakingValue = int64(2 * 10e8)
)

type BTCStakingTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *BTCStakingTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	// The e2e test flow is as follows:
	//
	// 1. Configure 1 chain with some validator nodes
	// 2. Execute various e2e tests
	s.configurer, err = configurer.NewBTCStakingConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

func (s *BTCStakingTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestCreateFinalityProviderAndDelegation is an end-to-end test for
// user story 1: user creates finality provider and BTC delegation
func (s *BTCStakingTestSuite) Test1CreateFinalityProviderAndDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	cacheFP = s.CreateNodeFP(nonValidatorNode)

	/*
		create a random BTC delegation under this finality provider
	*/
	// BTC staking params, BTC delegation key pairs and PoP
	params := nonValidatorNode.QueryBTCStakingParams()

	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	// NOTE: we use the node's address for the BTC delegation
	stakerAddr := sdk.MustAccAddressFromBech32(nonValidatorNode.PublicAddress)
	pop, err := bstypes.NewPoPBTC(stakerAddr, delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo, stakingTxInfo, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(nonValidatorNode, params, stakingTimeBlocks, cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delBTCSK)
	s.NoError(err)

	// submit the message for creating BTC delegation
	nonValidatorNode.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(delBTCPK),
		pop,
		stakingTxInfo,
		cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(unbondingTime),
		btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
		delUnbondingSlashingSig,
		nonValidatorNode.WalletName,
		false,
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	pendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(cacheFP.BtcPk.MarshalHex())
	s.Len(pendingDelSet, 1)
	pendingDels := pendingDelSet[0]
	s.Len(pendingDels.Dels, 1)
	s.Equal(delBTCPK.SerializeCompressed()[1:], pendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(pendingDels.Dels[0].CovenantSigs, 0)

	// check delegation
	delegation := nonValidatorNode.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(delegation.BtcDelegation.StakerAddr, nonValidatorNode.PublicAddress)
}

// Test2SubmitCovenantSignature is an end-to-end test for user
// story 2: covenant approves the BTC delegation
func (s *BTCStakingTestSuite) Test2SubmitCovenantSignature() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get last BTC delegation
	pendingDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(cacheFP.BtcPk.MarshalHex())
	s.Len(pendingDelsSet, 1)
	pendingDels := pendingDelsSet[0]
	s.Len(pendingDels.Dels, 1)
	pendingDelResp := pendingDels.Dels[0]
	pendingDel, err := ParseRespBTCDelToBTCDel(pendingDelResp)
	s.NoError(err)
	s.Len(pendingDel.CovenantSigs, 0)

	slashingTx := pendingDel.SlashingTx
	stakingTx := pendingDel.StakingTx

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(stakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash().String()

	params := nonValidatorNode.QueryBTCStakingParams()

	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	s.NoError(err)

	stakingInfo, err := pendingDel.GetStakingInfo(params, net)
	s.NoError(err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
		fpBTCPKs,
		stakingMsgTx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		slashingTx,
	)
	s.NoError(err)

	// cov Schnorr sigs on unbonding signature
	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	s.NoError(err)
	unbondingTx, err := bbn.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
	s.NoError(err)

	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covenantSKs,
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	s.NoError(err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(params, net)
	s.NoError(err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	s.NoError(err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	s.NoError(err)

	for i := 0; i < int(covenantQuorum); i++ {
		nonValidatorNode.AddCovenantSigs(
			covenantSlashingSigs[i].CovPk,
			stakingTxHash,
			covenantSlashingSigs[i].AdaptorSigs,
			bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
			covenantUnbondingSlashingSigs[i].AdaptorSigs,
		)
		// wait for a block so that above txs take effect
		nonValidatorNode.WaitForNextBlock()
	}

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()
	nonValidatorNode.WaitForNextBlock()

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(cacheFP.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(covenantQuorum))
}

// Test2CommitPublicRandomnessAndSubmitFinalitySignature is an end-to-end
// test for user story 3: finality provider commits public randomness and submits
// finality signature, such that blocks can be finalised.
func (s *BTCStakingTestSuite) Test3CommitPublicRandomnessAndSubmitFinalitySignature() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get activated height
	_, err = nonValidatorNode.QueryActivatedHeight()
	s.ErrorContains(err, bstypes.ErrBTCStakingNotActivated.Error())
	fps := nonValidatorNode.QueryFinalityProviders()
	s.Len(fps, 1)
	s.Zero(fps[0].VotingPower)

	/*
		commit a number of public randomness since activatedHeight
	*/
	// commit public randomness list
	numPubRand := uint64(100)
	commitStartHeight := uint64(1)
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, fpBTCSK, commitStartHeight, numPubRand)
	s.NoError(err)
	nonValidatorNode.CommitPubRandList(
		msgCommitPubRandList.FpBtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)

	// no reward gauge for finality provider and delegation yet
	fpBabylonAddr, err := sdk.AccAddressFromBech32(cacheFP.Addr)
	s.NoError(err)

	_, err = nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
	s.ErrorContains(err, itypes.ErrRewardGaugeNotFound.Error())
	delBabylonAddr := fpBabylonAddr

	// finalize epochs from 1 to the current epoch
	currentEpoch, err := nonValidatorNode.QueryCurrentEpoch()
	s.NoError(err)

	// wait until the end epoch is sealed
	s.Eventually(func() bool {
		resp, err := nonValidatorNode.QueryRawCheckpoint(currentEpoch)
		if err != nil {
			return false
		}
		return resp.Status == ckpttypes.Sealed
	}, time.Minute, time.Millisecond*50)
	nonValidatorNode.FinalizeSealedEpochs(1, currentEpoch)

	// ensure the committed epoch is finalized
	lastFinalizedEpoch := uint64(0)
	s.Eventually(func() bool {
		lastFinalizedEpoch, err = nonValidatorNode.QueryLastFinalizedEpoch()
		if err != nil {
			return false
		}
		return lastFinalizedEpoch >= currentEpoch
	}, time.Minute, time.Millisecond*50)

	// ensure btc staking is activated
	var activatedHeight uint64
	s.Eventually(func() bool {
		activatedHeight, err = nonValidatorNode.QueryActivatedHeight()
		if err != nil {
			return false
		}
		return activatedHeight > 0
	}, time.Minute, time.Millisecond*50)
	s.T().Logf("the activated height is %d", activatedHeight)

	/*
		submit finality signature
	*/
	// get block to vote
	blockToVote, err := nonValidatorNode.QueryBlock(int64(activatedHeight))
	s.NoError(err)
	appHash := blockToVote.AppHash

	idx := activatedHeight - commitStartHeight
	msgToSign := append(sdk.Uint64ToBigEndian(activatedHeight), appHash...)
	// generate EOTS signature
	sig, err := eots.Sign(fpBTCSK, randListInfo.SRList[idx], msgToSign)
	s.NoError(err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)
	// submit finality signature
	nonValidatorNode.AddFinalitySig(cacheFP.BtcPk, activatedHeight, &randListInfo.PRList[idx], *randListInfo.ProofList[idx].ToProto(), appHash, eotsSig)

	// ensure vote is eventually cast
	var finalizedBlocks []*ftypes.IndexedBlock
	s.Eventually(func() bool {
		finalizedBlocks = nonValidatorNode.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
		return len(finalizedBlocks) > 0
	}, time.Minute, time.Millisecond*50)
	s.Equal(activatedHeight, finalizedBlocks[0].Height)
	s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
	s.T().Logf("the block %d is finalized", activatedHeight)

	// ensure finality provider has received rewards after the block is finalised
	fpRewardGauges, err := nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
	s.NoError(err)
	fpRewardGauge, ok := fpRewardGauges[itypes.FinalityProviderType.String()]
	s.True(ok)
	s.True(fpRewardGauge.Coins.IsAllPositive())
	// ensure BTC delegation has received rewards after the block is finalised
	btcDelRewardGauges, err := nonValidatorNode.QueryRewardGauge(delBabylonAddr)
	s.NoError(err)
	btcDelRewardGauge, ok := btcDelRewardGauges[itypes.BTCDelegationType.String()]
	s.True(ok)
	s.True(btcDelRewardGauge.Coins.IsAllPositive())
	s.T().Logf("the finality provider received rewards for providing finality")
}

func (s *BTCStakingTestSuite) Test4WithdrawReward() {
	chainA := s.configurer.GetChainConfig(0)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// finality provider balance before withdraw
	fpBabylonAddr, err := sdk.AccAddressFromBech32(cacheFP.Addr)
	s.NoError(err)
	delBabylonAddr := fpBabylonAddr

	fpBalance, err := nonValidatorNode.QueryBalances(fpBabylonAddr.String())
	s.NoError(err)
	// finality provider reward gauge should not be fully withdrawn
	fpRgs, err := nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
	s.NoError(err)
	fpRg := fpRgs[itypes.FinalityProviderType.String()]
	s.T().Logf("finality provider's withdrawable reward before withdrawing: %s", fpRg.GetWithdrawableCoins().String())
	s.False(fpRg.IsFullyWithdrawn())

	// withdraw finality provider reward
	nonValidatorNode.WithdrawReward(itypes.FinalityProviderType.String(), initialization.ValidatorWalletName)
	nonValidatorNode.WaitForNextBlock()

	// balance after withdrawing finality provider reward
	fpBalance2, err := nonValidatorNode.QueryBalances(fpBabylonAddr.String())
	s.NoError(err)
	s.T().Logf("fpBalance2: %s; fpBalance: %s", fpBalance2.String(), fpBalance.String())
	s.True(fpBalance2.IsAllGT(fpBalance))
	// finality provider reward gauge should be fully withdrawn now
	fpRgs2, err := nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
	s.NoError(err)
	fpRg2 := fpRgs2[itypes.FinalityProviderType.String()]
	s.T().Logf("finality provider's withdrawable reward after withdrawing: %s", fpRg2.GetWithdrawableCoins().String())
	s.True(fpRg2.IsFullyWithdrawn())

	// BTC delegation balance before withdraw
	btcDelBalance, err := nonValidatorNode.QueryBalances(delBabylonAddr.String())
	s.NoError(err)
	// BTC delegation reward gauge should not be fully withdrawn
	btcDelRgs, err := nonValidatorNode.QueryRewardGauge(delBabylonAddr)
	s.NoError(err)
	btcDelRg := btcDelRgs[itypes.BTCDelegationType.String()]
	s.T().Logf("BTC delegation's withdrawable reward before withdrawing: %s", btcDelRg.GetWithdrawableCoins().String())
	s.False(btcDelRg.IsFullyWithdrawn())

	// withdraw BTC delegation reward
	nonValidatorNode.WithdrawReward(itypes.BTCDelegationType.String(), initialization.ValidatorWalletName)
	nonValidatorNode.WaitForNextBlock()

	// balance after withdrawing BTC delegation reward
	btcDelBalance2, err := nonValidatorNode.QueryBalances(delBabylonAddr.String())
	s.NoError(err)
	s.T().Logf("btcDelBalance2: %s; btcDelBalance: %s", btcDelBalance2.String(), btcDelBalance.String())
	s.True(btcDelBalance2.IsAllGT(btcDelBalance))
	// BTC delegation reward gauge should be fully withdrawn now
	btcDelRgs2, err := nonValidatorNode.QueryRewardGauge(delBabylonAddr)
	s.NoError(err)
	btcDelRg2 := btcDelRgs2[itypes.BTCDelegationType.String()]
	s.T().Logf("BTC delegation's withdrawable reward after withdrawing: %s", btcDelRg2.GetWithdrawableCoins().String())
	s.True(btcDelRg2.IsFullyWithdrawn())
}

// Test5SubmitStakerUnbonding is an end-to-end test for user unbonding
func (s *BTCStakingTestSuite) Test5SubmitStakerUnbonding() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(cacheFP.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)
	activeDels := activeDelsSet[0]
	s.Len(activeDels.Dels, 1)
	activeDelResp := activeDels.Dels[0]
	activeDel, err := ParseRespBTCDelToBTCDel(activeDelResp)
	s.NoError(err)
	s.NotNil(activeDel.CovenantSigs)

	// staking tx hash
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(activeDel.StakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash()

	// delegator signs unbonding tx
	params := nonValidatorNode.QueryBTCStakingParams()
	delUnbondingSig, err := activeDel.SignUnbondingTx(params, net, delBTCSK)
	s.NoError(err)

	// submit the message for creating BTC undelegation
	nonValidatorNode.BTCUndelegate(&stakingTxHash, delUnbondingSig)
	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	// Wait for unbonded delegations to be created
	var unbondedDelsResp []*bstypes.BTCDelegationResponse
	s.Eventually(func() bool {
		unbondedDelsResp = nonValidatorNode.QueryUnbondedDelegations()
		return len(unbondedDelsResp) > 0
	}, time.Minute, time.Second*2)

	unbondDel, err := ParseRespBTCDelToBTCDel(unbondedDelsResp[0])
	s.NoError(err)
	s.Equal(stakingTxHash, unbondDel.MustGetStakingTxHash())
}

// Test6MultisigBTCDelegation is an end-to-end test to create a BTC delegation
// with multisignature. It also utilizes the cacheFP populated at
// Test1CreateFinalityProviderAndDelegation.
func (s *BTCStakingTestSuite) Test6MultisigBTCDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	w1, w2, wMultisig := "multisig-holder-1", "multisig-holder-2", "multisig-2of2"

	nonValidatorNode.KeysAdd(w1)
	nonValidatorNode.KeysAdd(w2)
	// creates and fund multisig
	multisigAddr := nonValidatorNode.KeysAdd(wMultisig, []string{fmt.Sprintf("--multisig=%s,%s", w1, w2), "--multisig-threshold=2"}...)
	nonValidatorNode.BankSendFromNode(multisigAddr, "100000ubbn")

	// create a random BTC delegation under the cached finality provider
	// BTC staking params, BTC delegation key pairs and PoP
	params := nonValidatorNode.QueryBTCStakingParams()

	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	// NOTE: we use the multisig address for the BTC delegation
	multisigStakerAddr := sdk.MustAccAddressFromBech32(multisigAddr)
	pop, err := bstypes.NewPoPBTC(multisigStakerAddr, delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo, stakingTxInfo, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(nonValidatorNode, params, stakingTimeBlocks, cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delBTCSK)
	s.NoError(err)

	// submit the message for only generate the Tx to create BTC delegation
	jsonTx := nonValidatorNode.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(delBTCPK),
		pop,
		stakingTxInfo,
		cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(unbondingTime),
		btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
		delUnbondingSlashingSig,
		multisigAddr,
		true,
	)

	// write the tx to a file
	fullPathTxBTCDelegation := nonValidatorNode.WriteFile("tx.json", jsonTx)
	// signs the tx with the 2 wallets and the multisig and broadcast the tx
	nonValidatorNode.TxMultisignBroadcast(wMultisig, fullPathTxBTCDelegation, []string{w1, w2})

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	// check delegation with the multisig staker address exists.
	delegation := nonValidatorNode.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(multisigAddr, delegation.BtcDelegation.StakerAddr)
}

// Test7BTCDelegationFeeGrant is an end-to-end test to create a BTC delegation
// from a BTC delegator that does not have funds to pay for fees. It also
// utilizes the cacheFP populated at Test1CreateFinalityProviderAndDelegation.
func (s *BTCStakingTestSuite) Test7BTCDelegationFeeGrant() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	wGratee, wGranter := "grantee", "granter"
	feePayerAddr := sdk.MustAccAddressFromBech32(nonValidatorNode.KeysAdd(wGranter))
	granteeStakerAddr := sdk.MustAccAddressFromBech32(nonValidatorNode.KeysAdd(wGratee))

	feePayerBalanceBeforeBTCDel := sdk.NewCoin(params.DefaultBondDenom, sdkmath.NewInt(100000))
	fees := sdk.NewCoin(params.DefaultBondDenom, sdkmath.NewInt(50000))

	// fund the granter
	nonValidatorNode.BankSendFromNode(feePayerAddr.String(), feePayerBalanceBeforeBTCDel.String())

	// create a random BTC delegation under the cached finality provider
	// BTC staking btcStkParams, BTC delegation key pairs and PoP
	btcStkParams := nonValidatorNode.QueryBTCStakingParams()

	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	// NOTE: we use the grantee staker address for the BTC delegation PoP
	pop, err := bstypes.NewPoPBTC(granteeStakerAddr, delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16) - 5
	testStakingInfo, stakingTxInfo, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(nonValidatorNode, btcStkParams, stakingTimeBlocks, cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delBTCSK)
	s.NoError(err)

	// conceive the fee grant from the payer to the staker.
	nonValidatorNode.TxFeeGrant(feePayerAddr.String(), granteeStakerAddr.String(), fmt.Sprintf("--from=%s", wGranter))
	// wait for a block to take effect the fee grant tx.
	nonValidatorNode.WaitForNextBlock()

	// staker should not have any balance.
	stakerBalances, err := nonValidatorNode.QueryBalances(granteeStakerAddr.String())
	s.NoError(err)
	s.True(stakerBalances.IsZero())

	// submit the message to create BTC delegation
	nonValidatorNode.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(delBTCPK),
		pop,
		stakingTxInfo,
		cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(unbondingTime),
		btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
		delUnbondingSlashingSig,
		wGratee,
		false,
		fmt.Sprintf("--fee-granter=%s", feePayerAddr.String()),
		fmt.Sprintf("--fees=%s", fees.String()),
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	// check the delegation was success.
	delegation := nonValidatorNode.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(granteeStakerAddr.String(), delegation.BtcDelegation.StakerAddr)

	// verify the balances after the BTC delegation was submited
	// the staker should continue to have zero as balance.
	stakerBalances, err = nonValidatorNode.QueryBalances(granteeStakerAddr.String())
	s.NoError(err)
	s.True(stakerBalances.IsZero())

	// the fee payer should have the (feePayerBalanceBeforeBTCDel - fee) == currentBalance
	feePayerBalances, err := nonValidatorNode.QueryBalances(feePayerAddr.String())
	s.NoError(err)
	s.Equal(feePayerBalanceBeforeBTCDel.Sub(fees).String(), feePayerBalances.String())
}

// Test8BTCDelegationFeeGrantTyped is an end-to-end test to create a BTC delegation
// from a BTC delegator that does not have funds to pay for fees and explore scenarios
// to verify if the feeGrant is respected by the msg type and also spend limits. It also
// utilizes the cacheFP populated at Test1CreateFinalityProviderAndDelegation.
func (s *BTCStakingTestSuite) Test8BTCDelegationFeeGrantTyped() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	node, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	wGratee, wGranter := "staker", "feePayer"
	feePayerAddr := sdk.MustAccAddressFromBech32(node.KeysAdd(wGranter))
	granteeStakerAddr := sdk.MustAccAddressFromBech32(node.KeysAdd(wGratee))

	feePayerBalanceBeforeBTCDel := sdk.NewCoin(params.DefaultBondDenom, sdkmath.NewInt(100000))
	stakerBalance := sdk.NewCoin(params.DefaultBondDenom, sdkmath.NewInt(100))
	fees := sdk.NewCoin(params.DefaultBondDenom, sdkmath.NewInt(50000))

	// fund the granter and the staker
	node.BankSendFromNode(feePayerAddr.String(), feePayerBalanceBeforeBTCDel.String())
	node.BankSendFromNode(granteeStakerAddr.String(), stakerBalance.String())

	// create a random BTC delegation under the cached finality provider
	// BTC staking btcStkParams, BTC delegation key pairs and PoP
	btcStkParams := node.QueryBTCStakingParams()

	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	// NOTE: we use the grantee staker address for the BTC delegation PoP
	pop, err := bstypes.NewPoPBTC(granteeStakerAddr, delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16) - 2
	testStakingInfo, stakingTxInfo, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(node, btcStkParams, stakingTimeBlocks, cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delBTCSK)
	s.NoError(err)

	// conceive the fee grant from the payer to the staker only for one specific msg type.
	node.TxFeeGrant(
		feePayerAddr.String(), granteeStakerAddr.String(),
		fmt.Sprintf("--from=%s", wGranter),
		fmt.Sprintf("--%s=%s", feegrantcli.FlagSpendLimit, fees.String()),
		fmt.Sprintf("--%s=%s", feegrantcli.FlagAllowedMsgs, sdk.MsgTypeURL(&bstypes.MsgCreateBTCDelegation{})),
	)
	// wait for a block to take effect the fee grant tx.
	node.WaitForNextBlock()

	// tries to create a send transaction putting the freegranter as feepayer, it should FAIL
	// since we only gave grant for BTC delegation msgs.
	// TODO: Uncomment the next lines when issue: https://github.com/babylonlabs-io/babylon/issues/693
	// is fixed on cosmos-sdk side
	// outBuff, errBuff, err := node.BankSendOutput(
	// 	wGratee, node.PublicAddress, stakerBalance.String(),
	// 	fmt.Sprintf("--fee-granter=%s", feePayerAddr.String()),
	// )
	// outputStr := outBuff.String() + errBuff.String()
	// s.Require().Contains(outputStr, fmt.Sprintf("code: %d", feegrant.ErrMessageNotAllowed.ABCICode()))
	// s.Require().Contains(outputStr, feegrant.ErrMessageNotAllowed.Error())
	// s.Nil(err)

	// // staker should not have lost any balance.
	// stakerBalances, err := node.QueryBalances(granteeStakerAddr.String())
	// s.Require().NoError(err)
	// s.Require().Equal(stakerBalance.String(), stakerBalances.String())

	// submit the message to create BTC delegation using the fee grant
	// but putting as fee more than the spend limit
	// it should fail by exceeding the fee limit.
	// output := node.CreateBTCDelegation(
	// 	bbn.NewBIP340PubKeyFromBTCPK(delBTCPK),
	// 	pop,
	// 	stakingTxInfo,
	// 	cacheFP.BtcPk,
	// 	stakingTimeBlocks,
	// 	btcutil.Amount(stakingValue),
	// 	testStakingInfo.SlashingTx,
	// 	delegatorSig,
	// 	testUnbondingInfo.UnbondingTx,
	// 	testUnbondingInfo.SlashingTx,
	// 	uint16(unbondingTime),
	// 	btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
	// 	delUnbondingSlashingSig,
	// 	wGratee,
	// 	false,
	// 	fmt.Sprintf("--fee-granter=%s", feePayerAddr.String()),
	// 	fmt.Sprintf("--fees=%s", fees.Add(stakerBalance).String()),
	// )
	// s.Require().Contains(output, fmt.Sprintf("code: %d", feegrant.ErrFeeLimitExceeded.ABCICode()))
	// s.Require().Contains(output, feegrant.ErrFeeLimitExceeded.Error())

	// submit the message to create BTC delegation using the fee grant at the max of spend limit
	node.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(delBTCPK),
		pop,
		stakingTxInfo,
		cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(unbondingTime),
		btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
		delUnbondingSlashingSig,
		wGratee,
		false,
		fmt.Sprintf("--fee-granter=%s", feePayerAddr.String()),
		fmt.Sprintf("--fees=%s", fees.String()),
	)

	// wait for a block so that above txs take effect
	node.WaitForNextBlock()

	// check the delegation was success.
	delegation := node.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(granteeStakerAddr.String(), delegation.BtcDelegation.StakerAddr)

	// verify the balances after the BTC delegation was submited
	// the staker should continue to have zero as balance.
	stakerBalances, err := node.QueryBalances(granteeStakerAddr.String())
	s.NoError(err)
	s.Equal(stakerBalance.String(), stakerBalances.String())

	// the fee payer should have the (feePayerBalanceBeforeBTCDel - fee) == currentBalance
	feePayerBalances, err := node.QueryBalances(feePayerAddr.String())
	s.NoError(err)
	s.Equal(feePayerBalanceBeforeBTCDel.Sub(fees).String(), feePayerBalances.String())
}

// ParseRespsBTCDelToBTCDel parses an BTC delegation response to BTC Delegation
func ParseRespsBTCDelToBTCDel(resp *bstypes.BTCDelegatorDelegationsResponse) (btcDels *bstypes.BTCDelegatorDelegations, err error) {
	if resp == nil {
		return nil, nil
	}
	btcDels = &bstypes.BTCDelegatorDelegations{
		Dels: make([]*bstypes.BTCDelegation, len(resp.Dels)),
	}

	for i, delResp := range resp.Dels {
		del, err := ParseRespBTCDelToBTCDel(delResp)
		if err != nil {
			return nil, err
		}
		btcDels.Dels[i] = del
	}
	return btcDels, nil
}

// ParseRespBTCDelToBTCDel parses an BTC delegation response to BTC Delegation
func ParseRespBTCDelToBTCDel(resp *bstypes.BTCDelegationResponse) (btcDel *bstypes.BTCDelegation, err error) {
	stakingTx, err := hex.DecodeString(resp.StakingTxHex)
	if err != nil {
		return nil, err
	}

	delSig, err := bbn.NewBIP340SignatureFromHex(resp.DelegatorSlashSigHex)
	if err != nil {
		return nil, err
	}

	slashingTx, err := bstypes.NewBTCSlashingTxFromHex(resp.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	btcDel = &bstypes.BTCDelegation{
		StakerAddr:       resp.StakerAddr,
		BtcPk:            resp.BtcPk,
		FpBtcPkList:      resp.FpBtcPkList,
		StartHeight:      resp.StartHeight,
		EndHeight:        resp.EndHeight,
		TotalSat:         resp.TotalSat,
		StakingTx:        stakingTx,
		DelegatorSig:     delSig,
		StakingOutputIdx: resp.StakingOutputIdx,
		CovenantSigs:     resp.CovenantSigs,
		UnbondingTime:    resp.UnbondingTime,
		SlashingTx:       slashingTx,
	}

	if resp.UndelegationResponse != nil {
		ud := resp.UndelegationResponse
		unbondTx, err := hex.DecodeString(ud.UnbondingTxHex)
		if err != nil {
			return nil, err
		}

		slashTx, err := bstypes.NewBTCSlashingTxFromHex(ud.SlashingTxHex)
		if err != nil {
			return nil, err
		}

		delSlashingSig, err := bbn.NewBIP340SignatureFromHex(ud.DelegatorSlashingSigHex)
		if err != nil {
			return nil, err
		}

		btcDel.BtcUndelegation = &bstypes.BTCUndelegation{
			UnbondingTx:              unbondTx,
			CovenantUnbondingSigList: ud.CovenantUnbondingSigList,
			CovenantSlashingSigs:     ud.CovenantSlashingSigs,
			SlashingTx:               slashTx,
			DelegatorSlashingSig:     delSlashingSig,
		}

		if len(ud.DelegatorUnbondingSigHex) > 0 {
			delUnbondingSig, err := bbn.NewBIP340SignatureFromHex(ud.DelegatorUnbondingSigHex)
			if err != nil {
				return nil, err
			}
			btcDel.BtcUndelegation.DelegatorUnbondingSig = delUnbondingSig
		}
	}

	return btcDel, nil
}

func (s *BTCStakingTestSuite) equalFinalityProviderResp(fp *bstypes.FinalityProvider, fpResp *bstypes.FinalityProviderResponse) {
	s.Equal(fp.Description, fpResp.Description)
	s.Equal(fp.Commission, fpResp.Commission)
	s.Equal(fp.Addr, fpResp.Addr)
	s.Equal(fp.BtcPk, fpResp.BtcPk)
	s.Equal(fp.Pop, fpResp.Pop)
	s.Equal(fp.SlashedBabylonHeight, fpResp.SlashedBabylonHeight)
	s.Equal(fp.SlashedBtcHeight, fpResp.SlashedBtcHeight)
}

// CreateNodeFP creates a random finality provider.
func (s *BTCStakingTestSuite) CreateNodeFP(node *chain.NodeConfig) (newFP *bstypes.FinalityProvider) {
	// the node is the new FP
	nodeAddr, err := sdk.AccAddressFromBech32(node.PublicAddress)
	s.NoError(err)

	newFP, err = datagen.GenRandomFinalityProviderWithBTCBabylonSKs(r, fpBTCSK, nodeAddr)
	s.NoError(err)
	node.CreateFinalityProvider(newFP.Addr, newFP.BtcPk, newFP.Pop, newFP.Description.Moniker, newFP.Description.Identity, newFP.Description.Website, newFP.Description.SecurityContact, newFP.Description.Details, newFP.Commission)

	// wait for a block so that above txs take effect
	node.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFps := node.QueryFinalityProviders()
	s.Len(actualFps, 1)
	s.equalFinalityProviderResp(newFP, actualFps[0])

	return newFP
}

// CovenantBTCPKs returns the covenantBTCPks as slice from parameters
func CovenantBTCPKs(params *bstypes.Params) []*btcec.PublicKey {
	// get covenant BTC PKs
	covenantBTCPKs := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, covenantPK := range params.CovenantPks {
		covenantBTCPKs[i] = covenantPK.MustToBTCPK()
	}
	return covenantBTCPKs
}

// BTCStakingUnbondSlashInfo generate BTC information to create BTC delegation.
func (s *BTCStakingTestSuite) BTCStakingUnbondSlashInfo(
	node *chain.NodeConfig,
	params *bstypes.Params,
	stakingTimeBlocks uint16,
	fp *bstypes.FinalityProvider,
) (
	testStakingInfo *datagen.TestStakingSlashingInfo,
	stakingTxInfo *btcctypes.TransactionInfo,
	testUnbondingInfo *datagen.TestUnbondingSlashingInfo,
	delegatorSig *bbn.BIP340Signature,
) {
	covenantBTCPKs := CovenantBTCPKs(params)
	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	testStakingInfo = datagen.GenBTCStakingSlashingInfo(
		r,
		s.T(),
		net,
		delBTCSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	// submit staking tx to Bitcoin and get inclusion proof
	currentBtcTipResp, err := node.QueryTip()
	s.NoError(err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	stakingMsgTx := testStakingInfo.StakingTx

	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	node.InsertHeader(&blockWithStakingTx.HeaderBytes)
	// make block k-deep
	for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
		node.InsertNewEmptyBtcHeader(r)
	}
	stakingTxInfo = btcctypes.NewTransactionInfoFromSpvProof(blockWithStakingTx.SpvProof)

	// generate BTC undelegation stuff
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee
	testUnbondingInfo = datagen.GenBTCUnbondingSlashingInfo(
		r,
		s.T(),
		net,
		delBTCSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	stakingSlashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	delegatorSig, err = testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		datagen.StakingOutIdx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		delBTCSK,
	)
	s.NoError(err)

	return testStakingInfo, stakingTxInfo, testUnbondingInfo, delegatorSig
}
