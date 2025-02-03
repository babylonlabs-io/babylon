package e2e

import (
	"math"
	"math/rand"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/crypto/eots"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/x/incentive/types"
)

type BTCStakingPreApprovalTestSuite struct {
	suite.Suite

	r                    *rand.Rand
	net                  *chaincfg.Params
	fptBTCSK             *btcec.PrivateKey
	delBTCSK             *btcec.PrivateKey
	cacheFP              *bstypes.FinalityProvider
	cachedInclusionProof *bstypes.InclusionProof
	covenantSKs          []*btcec.PrivateKey
	covenantQuorum       uint32
	stakingValue         int64
	configurer           configurer.Configurer
}

func (s *BTCStakingPreApprovalTestSuite) SetupSuite() {
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
	s.configurer, err = configurer.NewBTCStakingConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

func (s *BTCStakingPreApprovalTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *BTCStakingPreApprovalTestSuite) Test1CreateFinalityProviderAndDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	s.cacheFP = CreateNodeFPFromNodeAddr(
		s.T(),
		s.r,
		s.fptBTCSK,
		nonValidatorNode,
	)

	/*
		create a random BTC delegation under this finality provider
	*/
	// BTC staking params, BTC delegation key pairs and PoP
	params := nonValidatorNode.QueryBTCStakingParams()

	// required unbonding time
	unbondingTime := params.UnbondingTimeBlocks

	// NOTE: we use the node's address for the BTC delegation
	stakerAddr := sdk.MustAccAddressFromBech32(nonValidatorNode.PublicAddress)
	pop, err := bstypes.NewPoPBTC(stakerAddr, s.delBTCSK)
	s.NoError(err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo, stakingTx, inclusionProof, testUnbondingInfo, delegatorSig := s.BTCStakingUnbondSlashInfo(nonValidatorNode, params, stakingTimeBlocks, s.cacheFP)
	s.cachedInclusionProof = inclusionProof

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(s.delBTCSK)
	s.NoError(err)

	// submit the message for creating BTC delegation
	nonValidatorNode.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(s.delBTCSK.PubKey()),
		pop,
		stakingTx,
		// We are passing `nil` as inclusion proof will be provided in separate tx
		nil,
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
		nonValidatorNode.WalletName,
		false,
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

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

func (s *BTCStakingPreApprovalTestSuite) Test2SubmitCovenantSignature() {
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
		nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
			nonValidatorNode.AddCovenantSigsFromVal(
				covenantSlashingSigs[i].CovPk,
				stakingTxHash,
				covenantSlashingSigs[i].AdaptorSigs,
				bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
				covenantUnbondingSlashingSigs[i].AdaptorSigs,
			)
			// wait for a block so that above txs take effect
			nonValidatorNode.WaitForNextBlock()
		}, true)
	}

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()
	nonValidatorNode.WaitForNextBlock()

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(s.cacheFP.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(s.covenantQuorum))
}

func (s *BTCStakingPreApprovalTestSuite) Test3SendStakingTransctionInclusionProof() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	verifiedDelegations := nonValidatorNode.QueryVerifiedDelegations()
	s.Len(verifiedDelegations, 1)

	btcDel, err := ParseRespBTCDelToBTCDel(verifiedDelegations[0])
	s.NoError(err)
	s.True(btcDel.HasCovenantQuorums(s.covenantQuorum))

	// staking tx hash
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(btcDel.StakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash()

	// make staking transacion inclusion block k-deep before submitting the inclusion proof
	for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
		nonValidatorNode.InsertNewEmptyBtcHeader(s.r)
	}

	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		nonValidatorNode.AddBTCDelegationInclusionProof(
			&stakingTxHash,
			s.cachedInclusionProof,
		)
		nonValidatorNode.WaitForNextBlock()
		nonValidatorNode.WaitForNextBlock()
	}, true)

	activeBTCDelegations := nonValidatorNode.QueryActiveDelegations()
	s.Len(activeBTCDelegations, 1)
	nonValidatorNode.WaitForNextBlock()
}

func (s *BTCStakingPreApprovalTestSuite) Test4CommitPublicRandomnessAndSubmitFinalitySignature() {
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
	nonValidatorNode.CommitPubRandList(
		msgCommitPubRandList.FpBtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)

	// no reward gauge for finality provider and delegation yet
	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.cacheFP.Addr)
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
	sig, err := eots.Sign(s.fptBTCSK, randListInfo.SRList[idx], msgToSign)
	s.NoError(err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)
	// submit finality signature
	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		nonValidatorNode.AddFinalitySigFromVal(s.cacheFP.BtcPk, activatedHeight, &randListInfo.PRList[idx], *randListInfo.ProofList[idx].ToProto(), appHash, eotsSig)

		// ensure vote is eventually cast
		var finalizedBlocks []*ftypes.IndexedBlock
		s.Eventually(func() bool {
			finalizedBlocks = nonValidatorNode.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
			return len(finalizedBlocks) > 0
		}, time.Minute, time.Millisecond*50)
		s.Equal(activatedHeight, finalizedBlocks[0].Height)
		s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
		s.T().Logf("the block %d is finalized", activatedHeight)
	}, true)

	// submit an invalid finality signature, and tx should NOT be refunded
	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		_, pk, err := datagen.GenRandomBTCKeyPair(s.r)
		s.NoError(err)
		btcPK := bbn.NewBIP340PubKeyFromBTCPK(pk)
		nonValidatorNode.AddFinalitySigFromVal(btcPK, activatedHeight, &randListInfo.PRList[idx], *randListInfo.ProofList[idx].ToProto(), appHash, eotsSig)
		nonValidatorNode.WaitForNextBlock()
	}, false)

	// ensure finality provider has received rewards after the block is finalised
	fpRewardGauges, err := nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
	s.NoError(err)
	fpRewardGauge, ok := fpRewardGauges[itypes.FinalityProviderType.String()]
	s.True(ok)
	s.True(fpRewardGauge.Coins.IsAllPositive())
	s.Require().Len(fpRewardGauge.Coins, 1)
	s.Require().True(!fpRewardGauge.Coins[0].IsZero())

	// ensure BTC delegation has received rewards after the block is finalised
	btcDelRewardGauges, err := nonValidatorNode.QueryRewardGauge(delBabylonAddr)
	s.NoError(err)
	btcDelRewardGauge, ok := btcDelRewardGauges[itypes.BTCDelegationType.String()]
	s.True(ok)
	s.True(btcDelRewardGauge.Coins.IsAllPositive())
	s.Require().Len(btcDelRewardGauge.Coins, 1)
	s.Require().True(!btcDelRewardGauge.Coins[0].IsZero())
	s.T().Logf("the finality provider received rewards for providing finality")
}

func (s *BTCStakingPreApprovalTestSuite) Test4WithdrawReward() {
	chainA := s.configurer.GetChainConfig(0)
	n, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	n.WithdrawRewardCheckingBalances(itypes.FinalityProviderType.String(), s.cacheFP.Addr)
	n.WithdrawRewardCheckingBalances(itypes.BTCDelegationType.String(), s.cacheFP.Addr)
}

// Test5SubmitStakerUnbonding is an end-to-end test for user unbonding
func (s *BTCStakingPreApprovalTestSuite) Test5SubmitStakerUnbonding() {
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
	activeDel, err := ParseRespBTCDelToBTCDel(activeDelResp)
	s.NoError(err)
	s.NotNil(activeDel.CovenantSigs)

	// staking tx hash
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(activeDel.StakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash()

	currentBtcTipResp, err := nonValidatorNode.QueryTip()
	s.NoError(err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	unbondingTx := activeDel.BtcUndelegation.UnbondingTx
	unbondingTxMsg, err := bbn.NewBTCTxFromBytes(unbondingTx)
	s.NoError(err)

	blockWithUnbondingTx := datagen.CreateBlockWithTransaction(s.r, currentBtcTip.Header.ToBlockHeader(), unbondingTxMsg)
	nonValidatorNode.InsertHeader(&blockWithUnbondingTx.HeaderBytes)
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithUnbondingTx.SpvProof)

	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		// submit the message for creating BTC undelegation
		nonValidatorNode.BTCUndelegate(
			&stakingTxHash,
			unbondingTxMsg,
			inclusionProof,
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

	unbondDel, err := ParseRespBTCDelToBTCDel(unbondedDelsResp[0])
	s.NoError(err)
	s.Equal(stakingTxHash, unbondDel.MustGetStakingTxHash())
}

func (s *BTCStakingPreApprovalTestSuite) BTCStakingUnbondSlashInfo(
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
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	stakingMsgTx := testStakingInfo.StakingTx

	blockWithStakingTx := datagen.CreateBlockWithTransaction(s.r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	node.InsertHeader(&blockWithStakingTx.HeaderBytes)

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
