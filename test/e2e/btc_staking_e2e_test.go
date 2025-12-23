package e2e

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	govv1 "cosmossdk.io/api/cosmos/gov/v1"
	sdkmath "cosmossdk.io/math"
	feegrantcli "cosmossdk.io/x/feegrant/client/cli"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

type BTCStakingTestSuite struct {
	suite.Suite

	r              *rand.Rand
	net            *chaincfg.Params
	fptBTCSK       *btcec.PrivateKey
	delBTCSK       *btcec.PrivateKey
	cacheFP        *bstypes.FinalityProvider
	covenantSKs    []*btcec.PrivateKey
	covenantQuorum uint32
	stakingValue   int64
	configurer     configurer.Configurer

	feePayerAddr string
}

func (s *BTCStakingTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.fptBTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.delBTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.stakingValue = int64(2 * 10e8)
	covenantSKs, _, covenantQuorum := bstypes.DefaultCovenantCommittee()
	s.covenantSKs = covenantSKs
	s.covenantQuorum = covenantQuorum

	// The e2e test flow is as follows:
	//
	// 1. Configure 1 chain with some validator nodes
	// 2. Execute various e2e tests
	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

// TestCreateFinalityProviderAndDelegation is an end-to-end test for
// user story 1: user creates finality provider and BTC delegation
func (s *BTCStakingTestSuite) Test1CreateFinalityProviderAndDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	s.cacheFP = chain.CreateFpFromNodeAddr(
		s.T(),
		s.r,
		s.fptBTCSK,
		nonValidatorNode,
	)

	/*
		create a random BTC delegation under this finality provider
	*/

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)

	// NOTE: we use the node's address for the BTC delegation
	testStakingInfo := nonValidatorNode.CreateBTCDelegationAndCheck(
		s.r,
		s.T(),
		s.net,
		nonValidatorNode.WalletName,
		s.cacheFP,
		s.delBTCSK,
		nonValidatorNode.PublicAddress,
		stakingTimeBlocks,
		s.stakingValue,
	)

	pendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(s.cacheFP.BtcPk.MarshalHex())
	s.Len(pendingDelSet, 1)
	pendingDels := pendingDelSet[0]
	s.Len(pendingDels.Dels, 1)
	s.Equal(s.delBTCSK.PubKey().SerializeCompressed()[1:], pendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
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
	pendingDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(s.cacheFP.BtcPk.MarshalHex())
	s.Len(pendingDelsSet, 1)
	pendingDels := pendingDelsSet[0]
	s.Len(pendingDels.Dels, 1)
	pendingDelResp := pendingDels.Dels[0]
	pendingDel, err := tkeeper.ParseRespBTCDelToBTCDel(pendingDelResp)
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

	stakingInfo, err := pendingDel.GetStakingInfo(params, s.net)
	s.NoError(err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		s.covenantSKs,
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
		s.covenantSKs,
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	s.NoError(err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(params, s.net)
	s.NoError(err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	s.NoError(err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		s.covenantSKs,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	s.NoError(err)

	for i := 0; i < int(s.covenantQuorum); i++ {
		// after adding the covenant signatures it panics with "BTC delegation rewards tracker has a negative amount of TotalActiveSat"
		nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
			// add covenant sigs
			nonValidatorNode.AddCovenantSigsFromVal(
				covenantSlashingSigs[i].CovPk,
				stakingTxHash,
				covenantSlashingSigs[i].AdaptorSigs,
				bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
				covenantUnbondingSlashingSigs[i].AdaptorSigs,
				nil,
			)
			// wait for a block so that above txs take effect
			nonValidatorNode.WaitForNextBlock()
		}, true)
	}

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlocks(2)

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(s.cacheFP.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)

	activeDels, err := chain.ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(s.covenantQuorum, 0))
}

// Test3CommitPublicRandomnessAndSubmitFinalitySignature is an end-to-end
// test for user story 3: finality provider commits public randomness and submits
// finality signature, such that blocks can be finalised.
func (s *BTCStakingTestSuite) Test3CommitPublicRandomnessAndSubmitFinalitySignature() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	/*
		commit a number of public randomness
	*/
	// commit public randomness list
	numPubRand := uint64(100)
	commitStartHeight := uint64(1)

	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fptBTCSK, commitStartHeight, numPubRand)
	s.NoError(err)
	nonValidatorNode.CommitPubRandListFromNode(
		msgCommitPubRandList.FpBtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)

	// Query the public randomness commitment for the finality provider
	var commitEpoch uint64
	s.Eventually(func() bool {
		pubRandCommitMap := nonValidatorNode.QueryListPubRandCommit(msgCommitPubRandList.FpBtcPk)
		if len(pubRandCommitMap) == 0 {
			return false
		}
		for _, commit := range pubRandCommitMap {
			commitEpoch = commit.EpochNum
		}
		return true
	}, time.Minute, time.Second)

	s.T().Logf("Successfully queried public randomness commitment for finality provider at epoch %d", commitEpoch)

	// no reward gauge for finality provider and delegation yet
	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.cacheFP.Addr)
	s.NoError(err)

	_, err = nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
	s.ErrorContains(err, itypes.ErrRewardGaugeNotFound.Error())
	delBabylonAddr := fpBabylonAddr

	nonValidatorNode.WaitUntilCurrentEpochIsSealedAndFinalized(1)

	// ensure btc staking is activated
	// check how this does not errors out
	activatedHeight := nonValidatorNode.WaitFinalityIsActivated()

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
	sig, err := eots.Sign(s.fptBTCSK, randListInfo.SRList[idx], msgToSign)
	s.NoError(err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)

	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		// submit finality signature
		nonValidatorNode.AddFinalitySigFromVal(s.cacheFP.BtcPk, activatedHeight, &randListInfo.PRList[idx], *randListInfo.ProofList[idx].ToProto(), appHash, eotsSig)

		// ensure vote is eventually cast
		var finalizedBlocks []*ftypes.IndexedBlock
		s.Require().Eventually(func() bool {
			finalizedBlocks = nonValidatorNode.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
			return len(finalizedBlocks) > 0
		}, time.Minute, time.Millisecond*50)
		s.Equal(activatedHeight, finalizedBlocks[0].Height)
		s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
		s.T().Logf("the block %d is finalized", activatedHeight)
	}, true)

	finalityParams := nonValidatorNode.QueryFinalityParams()
	nonValidatorNode.WaitForNextBlocks(uint64(finalityParams.FinalitySigTimeout))

	// ensure finality provider has received rewards after the block is finalised
	fpRewardGauges, err := nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
	s.NoError(err)
	fpRewardGauge, ok := fpRewardGauges[itypes.FINALITY_PROVIDER.String()]
	s.True(ok)
	s.True(fpRewardGauge.Coins.IsAllPositive())
	// ensure BTC delegation has received rewards after the block is finalised
	btcDelRewardGauges, err := nonValidatorNode.QueryRewardGauge(delBabylonAddr)
	s.NoError(err)
	btcDelRewardGauge, ok := btcDelRewardGauges[itypes.BTC_STAKER.String()]
	s.True(ok)
	s.True(btcDelRewardGauge.Coins.IsAllPositive())
	s.T().Logf("the finality provider received rewards for providing finality")
}

func (s *BTCStakingTestSuite) Test4WithdrawReward() {
	chainA := s.configurer.GetChainConfig(0)
	n, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	n.WithdrawRewardCheckingBalances(itypes.FINALITY_PROVIDER.String(), s.cacheFP.Addr)
	n.WithdrawRewardCheckingBalances(itypes.BTC_STAKER.String(), s.cacheFP.Addr)
}

// Test5SubmitStakerUnbonding is an end-to-end test for user unbonding
func (s *BTCStakingTestSuite) Test5SubmitStakerUnbonding() {
	s.T().Logf("logging Test5SubmitStakerUnbonding")
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(s.cacheFP.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)
	activeDels := activeDelsSet[0]
	s.Len(activeDels.Dels, 1)
	activeDelResp := activeDels.Dels[0]
	activeDel, err := tkeeper.ParseRespBTCDelToBTCDel(activeDelResp)
	s.NoError(err)
	s.NotNil(activeDel.CovenantSigs)

	// staking tx hash
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(activeDel.StakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash()

	currentBtcTipResp, err := nonValidatorNode.QueryTip()
	s.NoError(err)
	currentBtcTip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	unbondingTx := activeDel.MustGetUnbondingTx()

	_, unbondingTxMsg := datagen.AddWitnessToUnbondingTx(
		s.T(),
		stakingMsgTx.TxOut[activeDel.StakingOutputIdx],
		s.delBTCSK,
		s.covenantSKs,
		s.covenantQuorum,
		[]*btcec.PublicKey{s.cacheFP.BtcPk.MustToBTCPK()},
		uint16(activeDel.GetStakingTime()),
		int64(activeDel.TotalSat),
		unbondingTx,
		s.net,
	)

	blockWithUnbondingTx := datagen.CreateBlockWithTransaction(s.r, currentBtcTip.Header.ToBlockHeader(), unbondingTxMsg)
	nonValidatorNode.InsertHeader(&blockWithUnbondingTx.HeaderBytes)
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithUnbondingTx.SpvProof)

	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		// submit the message for creating BTC undelegation
		nonValidatorNode.BTCUndelegate(
			&stakingTxHash,
			unbondingTxMsg,
			inclusionProof,
			[]*wire.MsgTx{
				stakingMsgTx,
			},
		)
		// wait for a block so that above txs take effect
		nonValidatorNode.WaitForNextBlock()
	}, true)

	// Wait for unbonded delegations to be created
	var unbondedDelsResp []*bstypes.BTCDelegationResponse
	s.Eventually(func() bool {
		unbondedDelsResp = nonValidatorNode.QueryUnbondedDelegations()
		return len(unbondedDelsResp) > 0
	}, time.Minute, time.Second*2)

	unbondDel, err := tkeeper.ParseRespBTCDelToBTCDel(unbondedDelsResp[0])
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
	nonValidatorNode.BankSendFromNode(multisigAddr, "10000000ubbn")

	// create a random BTC delegation under the cached finality provider
	// BTC staking params, BTC delegation key pairs and PoP
	params := nonValidatorNode.QueryBTCStakingParams()

	// required unbonding time
	unbondingTime := params.UnbondingTimeBlocks

	// NOTE: we use the multisig address for the BTC delegation
	multisigStakerAddr := sdk.MustAccAddressFromBech32(multisigAddr)
	pop, err := datagen.NewPoPBTC(multisigStakerAddr, s.delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo, stakingTx, inclusionProof, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(nonValidatorNode, params, stakingTimeBlocks, s.cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(s.delBTCSK)
	s.NoError(err)

	// submit the message for only generate the Tx to create BTC delegation
	jsonTx := nonValidatorNode.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(s.delBTCSK.PubKey()),
		pop,
		stakingTx,
		inclusionProof,
		s.cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(s.stakingValue),
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

	feePayerBalanceBeforeBTCDel := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(5000000))

	// fund the granter
	nonValidatorNode.BankSendFromNode(feePayerAddr.String(), feePayerBalanceBeforeBTCDel.String())

	// create a random BTC delegation under the cached finality provider
	// BTC staking btcStkParams, BTC delegation key pairs and PoP
	btcStkParams := nonValidatorNode.QueryBTCStakingParams()

	// required unbonding time
	unbondingTime := btcStkParams.UnbondingTimeBlocks

	// NOTE: we use the grantee staker address for the BTC delegation PoP
	pop, err := datagen.NewPoPBTC(granteeStakerAddr, s.delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16) - 5
	testStakingInfo, stakingTx, inclusionProof, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(nonValidatorNode, btcStkParams, stakingTimeBlocks, s.cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(s.delBTCSK)
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
		bbn.NewBIP340PubKeyFromBTCPK(s.delBTCSK.PubKey()),
		pop,
		stakingTx,
		inclusionProof,
		s.cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(s.stakingValue),
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
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	// check the delegation was success.
	delegation := nonValidatorNode.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(granteeStakerAddr.String(), delegation.BtcDelegation.StakerAddr)

	// verify the balances after the BTC delegation was submitted
	// the staker should continue to have zero as balance.
	stakerBalances, err = nonValidatorNode.QueryBalances(granteeStakerAddr.String())
	s.NoError(err)
	s.True(stakerBalances.IsZero())

	// the fee payer should have the feePayerBalanceBeforeBTCDel > currentBalance
	feePayerBalances, err := nonValidatorNode.QueryBalances(feePayerAddr.String())
	s.NoError(err)
	s.True(feePayerBalanceBeforeBTCDel.Amount.GT(feePayerBalances.AmountOf(appparams.BaseCoinUnit)))
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
	s.feePayerAddr = node.KeysAdd(wGranter)
	feePayerAddr := sdk.MustAccAddressFromBech32(s.feePayerAddr)
	granteeStakerAddr := sdk.MustAccAddressFromBech32(node.KeysAdd(wGratee))

	feePayerBalanceBeforeBTCDel := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(4000000))
	stakerBalance := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000))
	fees := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(500000))

	// fund the granter and the staker
	node.BankSendFromNode(feePayerAddr.String(), feePayerBalanceBeforeBTCDel.String())
	node.BankSendFromNode(granteeStakerAddr.String(), stakerBalance.String())

	// create a random BTC delegation under the cached finality provider
	// BTC staking btcStkParams, BTC delegation key pairs and PoP
	btcStkParams := node.QueryBTCStakingParams()

	// required unbonding time
	unbondingTime := btcStkParams.UnbondingTimeBlocks

	// NOTE: we use the grantee staker address for the BTC delegation PoP
	pop, err := datagen.NewPoPBTC(granteeStakerAddr, s.delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16) - 2
	testStakingInfo, stakingTx, inclusionProof, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(node, btcStkParams, stakingTimeBlocks, s.cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(s.delBTCSK)
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
	// TODO: Uncomment the next lines when issue: https://github.com/babylonlabs-io/babylon/v4/issues/693
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
		bbn.NewBIP340PubKeyFromBTCPK(s.delBTCSK.PubKey()),
		pop,
		stakingTx,
		inclusionProof,
		s.cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(s.stakingValue),
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
	)

	// wait for a block so that above txs take effect
	node.WaitForNextBlock()

	// check the delegation was success.
	delegation := node.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(granteeStakerAddr.String(), delegation.BtcDelegation.StakerAddr)

	// verify the balances after the BTC delegation was submitted
	// the staker should continue to have zero as balance.
	stakerBalances, err := node.QueryBalances(granteeStakerAddr.String())
	s.NoError(err)
	s.Equal(stakerBalance.String(), stakerBalances.String())

	// the fee payer should have the feePayerBalanceBeforeBTCDel > currentBalance
	feePayerBalances, err := node.QueryBalances(feePayerAddr.String())
	s.NoError(err)
	s.True(feePayerBalanceBeforeBTCDel.Amount.GT(feePayerBalances.AmountOf(appparams.BaseCoinUnit)))
}

func (s *BTCStakingTestSuite) Test9BlockBankSendAndBTCDelegate() {
	c := s.configurer.GetChainConfig(0)
	n, err := c.GetNodeAtIndex(2)
	s.NoError(err)

	propID := n.TxGovPropSubmitProposal("/govProps/block-bank-send.json", n.WalletName)
	c.TxGovVoteFromAllNodes(propID, govv1.VoteOption_VOTE_OPTION_YES)

	s.Eventually(func() bool {
		sendEnabled, err := n.QueryBankSendEnabled(appparams.DefaultBondDenom)
		if err != nil {
			s.T().Logf("not possible to query if denom is enabled: %s", err.Error())
			return false
		}

		if len(sendEnabled) != 1 {
			s.T().Log("send enabled returned more than it should")
			return false
		}
		bondDenomSendEnabled := sendEnabled[0]
		return strings.EqualFold(bondDenomSendEnabled.Denom, appparams.DefaultBondDenom) &&
			bondDenomSendEnabled.Enabled == false
	}, time.Minute*5, time.Second*5)

	stakerNoFundsAddr := n.KeysAdd("out-of-funds-btc-staker")

	// create a random BTC delegation under the cached finality provider
	// BTC staking btcStkParams, BTC delegation key pairs and PoP
	btcStkParams := n.QueryBTCStakingParams()

	// required unbonding time
	unbondingTime := btcStkParams.UnbondingTimeBlocks

	// NOTE: we use the grantee staker address for the BTC delegation PoP
	pop, err := datagen.NewPoPBTC(sdk.MustAccAddressFromBech32(stakerNoFundsAddr), s.delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16) - 2
	testStakingInfo, stakingTx, inclusionProof, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(n, btcStkParams, stakingTimeBlocks, s.cacheFP)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(s.delBTCSK)
	s.NoError(err)

	fees := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(50000))

	// conceive the fee grant from the payer to the staker only for one specific msg type.
	n.TxFeeGrant(
		s.feePayerAddr, stakerNoFundsAddr,
		fmt.Sprintf("--from=%s", s.feePayerAddr),
		fmt.Sprintf("--%s=%s", feegrantcli.FlagSpendLimit, fees.String()),
		fmt.Sprintf("--%s=%s", feegrantcli.FlagAllowedMsgs, sdk.MsgTypeURL(&bstypes.MsgCreateBTCDelegation{})),
	)
	// wait for a block to take effect the fee grant tx.
	n.WaitForNextBlock()

	balancesFeePayerBeforeBtcDel, err := n.QueryBalances(s.feePayerAddr)
	s.NoError(err)
	balancesStakerBeforeBtcDel, err := n.QueryBalances(stakerNoFundsAddr)
	s.NoError(err)

	// submit the message to create BTC delegation using the fee grant at the max of spend limit
	btcPK := bbn.NewBIP340PubKeyFromBTCPK(s.delBTCSK.PubKey())
	outStrBtcDel := n.CreateBTCDelegation(
		btcPK,
		pop,
		stakingTx,
		inclusionProof,
		s.cacheFP.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(s.stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(unbondingTime),
		btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
		delUnbondingSlashingSig,
		stakerNoFundsAddr,
		false,
		fmt.Sprintf("--fee-granter=%s", s.feePayerAddr),
	)

	txHashBtcDel := chain.GetTxHashFromOutput(outStrBtcDel)

	// wait for a block so that above txs take effect
	n.WaitForNextBlock()

	_, txResp := n.QueryTx(txHashBtcDel)
	txFeesPaid := txResp.AuthInfo.Fee.Amount

	// verify the expected balances after the btc delegation gets included
	balancesFeePayerAfterBtcDel, err := n.QueryBalances(s.feePayerAddr)
	s.NoError(err)
	balancesStakerAfterBtcDel, err := n.QueryBalances(stakerNoFundsAddr)
	s.NoError(err)

	s.Equal(balancesStakerBeforeBtcDel.String(), balancesStakerAfterBtcDel.String())
	s.Equal(balancesFeePayerBeforeBtcDel.String(), balancesFeePayerAfterBtcDel.Add(txFeesPaid...).String())

	// check that the delegation succeeded
	delegation := n.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(stakerNoFundsAddr, delegation.BtcDelegation.StakerAddr)
}

// CreateNodeFP creates a random finality provider.
func CreateNodeFP(
	t *testing.T,
	r *rand.Rand,
	fpSk *btcec.PrivateKey,
	node *chain.NodeConfig,
	fpAddr string,
) (newFP *bstypes.FinalityProvider) {
	// the node is the new FP
	nodeAddr, err := sdk.AccAddressFromBech32(fpAddr)
	require.NoError(t, err)

	newFP, err = datagen.GenRandomFinalityProviderWithBTCBabylonSKs(r, fpSk, nodeAddr)
	require.NoError(t, err)

	previousFps := node.QueryFinalityProviders()

	// use a higher commission to ensure the reward is more than tx fee of a finality sig
	commission := sdkmath.LegacyNewDecWithPrec(20, 2)
	newFP.Commission = &commission
	node.CreateFinalityProvider(newFP.Addr, newFP.BtcPk, newFP.Pop, newFP.Description.Moniker, newFP.Description.Identity, newFP.Description.Website, newFP.Description.SecurityContact, newFP.Description.Details, newFP.Commission, newFP.CommissionInfo.MaxRate, newFP.CommissionInfo.MaxChangeRate)

	// wait for a block so that above txs take effect
	node.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFps := node.QueryFinalityProviders()
	require.Len(t, actualFps, len(previousFps)+1)

	for _, fpResp := range actualFps {
		if !strings.EqualFold(fpResp.Addr, newFP.Addr) {
			continue
		}
		chain.EqualFinalityProviderResp(t, newFP, fpResp)
		return newFP
	}

	return nil
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
	stakingTx []byte,
	txInclusionProof *bstypes.InclusionProof,
	testUnbondingInfo *datagen.TestUnbondingSlashingInfo,
	delegatorSig *bbn.BIP340Signature,
) {
	covenantBTCPKs := CovenantBTCPKs(params)
	// required unbonding time
	unbondingTime := params.UnbondingTimeBlocks

	testStakingInfo = datagen.GenBTCStakingSlashingInfo(
		s.r,
		s.T(),
		s.net,
		s.delBTCSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		s.covenantQuorum,
		stakingTimeBlocks,
		s.stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(unbondingTime),
	)

	// submit staking tx to Bitcoin and get inclusion proof
	currentBtcTipResp, err := node.QueryTip()
	s.NoError(err)
	currentBtcTip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	stakingMsgTx := testStakingInfo.StakingTx

	blockWithStakingTx := datagen.CreateBlockWithTransaction(s.r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	node.InsertHeader(&blockWithStakingTx.HeaderBytes)
	// make block k-deep
	for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
		node.InsertNewEmptyBtcHeader(s.r)
	}
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithStakingTx.SpvProof)

	// generate BTC undelegation stuff
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := s.stakingValue - datagen.UnbondingTxFee
	testUnbondingInfo = datagen.GenBTCUnbondingSlashingInfo(
		s.r,
		s.T(),
		s.net,
		s.delBTCSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		s.covenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(unbondingTime),
	)

	stakingSlashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	delegatorSig, err = testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		datagen.StakingOutIdx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		s.delBTCSK,
	)
	s.NoError(err)

	return testStakingInfo, blockWithStakingTx.SpvProof.BtcTransaction, inclusionProof, testUnbondingInfo, delegatorSig
}
