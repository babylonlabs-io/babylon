package e2e

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/suite"

	govv1 "cosmossdk.io/api/cosmos/gov/v1"
	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/config"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	v1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

type GovFinalityResume struct {
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
}

func (s *GovFinalityResume) SetupSuite() {
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

func (s *GovFinalityResume) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// Test1CreateFpAndDel is an end-to-end test for
// user story 1: user creates finality provider and BTC delegation
func (s *GovFinalityResume) Test1CreateFpAndDel() {
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
func (s *GovFinalityResume) Test2SubmitCovenantSignature() {
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
func (s *GovFinalityResume) Test3CommitPublicRandomnessAndSubmitFinalitySignature() {
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
		}, time.Minute, time.Millisecond*50, "It didn't finalized any block")
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

func (s *GovFinalityResume) Test4UpgradeResumeFinality() {
	c := s.configurer.GetChainConfig(0)
	c.WaitUntilHeight(1)

	n, err := c.GetNodeAtIndex(2)
	s.NoError(err)

	propPath, err := s.PropPath()
	s.NoError(err)

	cdc := app.NewTmpBabylonApp().AppCodec()

	haltBlockHeight, err := n.QueryActivatedHeight()
	s.NoError(err)
	// increase one block, since we only voted for the activated height
	haltBlockHeight++

	prop, msg, err := parseGovPropResumeFinalityToFileFromFile(cdc, propPath)
	s.NoError(err)

	btcPk := s.cacheFP.BtcPk.MarshalHex()
	msg.FpPksHex = []string{btcPk}
	msg.HaltingHeight = uint32(haltBlockHeight)

	err = WriteGovPropResumeFinalityToFile(cdc, propPath, *prop, *msg)
	s.NoError(err)

	// uses relative path to submit the prop
	propID := n.TxGovPropSubmitProposal(config.GovPropResumeFinality, n.WalletName)
	c.TxGovVoteFromAllNodes(propID, govv1.VoteOption_VOTE_OPTION_YES)

	fp := n.QueryFinalityProvider(btcPk)
	s.Require().False(fp.Jailed)

	s.Eventually(func() bool {
		propResp := n.QueryProposal(propID)
		if propResp.Proposal.Status != v1beta1.StatusPassed {
			s.T().Logf("Proposal %d did not passed: %s", propID, propResp.Proposal.Status.String())
			return false
		}
		return propResp.Proposal.Status == v1beta1.StatusPassed
	}, time.Minute*5, time.Second*6)

	n.WaitForNextBlock()

	fp = n.QueryFinalityProvider(btcPk)
	s.Require().True(fp.Jailed)
}

// WriteGovPropResumeFinalityToFile loads from the file the Upgrade msg as json.
func WriteGovPropResumeFinalityToFile(cdc codec.Codec, propFilePath string, prop chain.Proposal, msg ftypes.MsgResumeFinalityProposal) error {
	bz, err := cdc.MarshalInterfaceJSON(&msg)
	if err != nil {
		return err
	}
	prop.Messages = []json.RawMessage{bz}

	return chain.WriteProposalToFile(cdc, propFilePath, prop)
}

// parseGovPropResumeFinalityToFileFromFile loads from the file and parse it to the upgrade msg.
func parseGovPropResumeFinalityToFileFromFile(cdc codec.Codec, propFilePath string) (*chain.Proposal, *ftypes.MsgResumeFinalityProposal, error) {
	prop, msgs, _, err := chain.ParseSubmitProposal(cdc, propFilePath)
	if err != nil {
		return nil, nil, err
	}

	msg, ok := msgs[0].(*ftypes.MsgResumeFinalityProposal)
	if !ok {
		return nil, nil, fmt.Errorf("unable to parse msg to ftypes.MsgResumeFinalityProposal")
	}
	return &prop, msg, nil
}

// PropPath returns the local full path of the upgrade file
func (s *GovFinalityResume) PropPath() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(pwd, config.GovPropResumeFinality), nil
}
