package e2e

import (
	"math"
	"math/rand"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

type CostakingTestSuite struct {
	suite.Suite

	r   *rand.Rand
	net *chaincfg.Params

	covenantSKs    []*btcec.PrivateKey
	covenantQuorum uint32

	configurer configurer.Configurer
}

func (s *CostakingTestSuite) SetupSuite() {
	s.T().Log("setting up costaking e2e test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.covenantSKs, _, s.covenantQuorum = bstypes.DefaultCovenantCommittee()

	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

func (s *CostakingTestSuite) TearDownSuite() {
	if s.configurer != nil {
		if err := s.configurer.ClearResources(); err != nil {
			s.T().Logf("error clearing resources: %v", err)
		}
	}
}

func (s *CostakingTestSuite) TestFinalityProviderExit() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)

	delegatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	validators, err := delegatorNode.QueryValidators()
	s.NoError(err)
	s.Require().NotEmpty(validators)
	targetValidator := validators[0]

	babyDelegation := "20000000ubbn"
	txHash := delegatorNode.Delegate(delegatorNode.WalletName, targetValidator.OperatorAddress, babyDelegation, "--gas=500000")
	chainA.WaitForNumHeights(2)
	res, _ := delegatorNode.QueryTx(txHash)
	s.Equal(res.Code, uint32(0), res.RawLog)

	_, err = delegatorNode.WaitForNextEpoch()
	s.NoError(err)

	fpSk, _, _ := datagen.GenRandomBTCKeyPair(s.r)
	fp := chain.CreateFpFromNodeAddr(s.T(), s.r, fpSk, delegatorNode)

	numPubRand := uint64(1000)
	commitStartHeight := uint64(1)
	_, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, fpSk, commitStartHeight, numPubRand)
	s.NoError(err)
	delegatorNode.CommitPubRandListFromNode(
		msgCommitPubRandList.FpBtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)

	var commitEpoch uint64
	s.Require().Eventually(func() bool {
		pubRandCommitMap := delegatorNode.QueryListPubRandCommit(msgCommitPubRandList.FpBtcPk)
		if len(pubRandCommitMap) == 0 {
			return false
		}
		for _, commit := range pubRandCommitMap {
			commitEpoch = commit.EpochNum
			break
		}
		return true
	}, time.Minute, time.Second, "finality provider should have public randomness committed")
	s.T().Logf("fp pub rand commitment stored for epoch %d", commitEpoch)

	lastFinalizedEpoch := delegatorNode.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	s.Require().GreaterOrEqual(lastFinalizedEpoch, commitEpoch, "finalized epoch must include fp pub rand commit")

	_, err = delegatorNode.WaitForNextEpoch()
	s.NoError(err)

	btcDelegatorSK, _, _ := datagen.GenRandomBTCKeyPair(s.r)
	delegatorAddr := sdk.MustAccAddressFromBech32(delegatorNode.PublicAddress)
	pop, err := datagen.NewPoPBTC(delegatorAddr, btcDelegatorSK)
	s.NoError(err)

	params := delegatorNode.QueryBTCStakingParams()
	stakingTimeBlocks := uint16(math.MaxUint16)
	stakingValue := int64(2 * 10e8)

	testStakingInfo, stakingTx, stakingInclusionProof, testUnbondingInfo, delegatorSig := delegatorNode.BTCStakingUnbondSlashInfo(
		s.r,
		s.T(),
		s.net,
		params,
		fp,
		btcDelegatorSK,
		stakingTimeBlocks,
		stakingValue,
	)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(btcDelegatorSK)
	s.NoError(err)

	delegatorNode.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(btcDelegatorSK.PubKey()),
		pop,
		stakingTx,
		stakingInclusionProof,
		fp.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(params.UnbondingTimeBlocks),
		btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
		delUnbondingSlashingSig,
		delegatorNode.WalletName,
		false,
	)

	delegatorNode.WaitForNextBlock()

	pendingSet := delegatorNode.QueryFinalityProviderDelegations(fp.BtcPk.MarshalHex())
	s.Require().Len(pendingSet, 1)
	pendingResp := pendingSet[0]
	s.Require().Len(pendingResp.Dels, 1)
	pendingDel, err := chain.ParseRespBTCDelToBTCDel(pendingResp.Dels[0])
	s.Require().NoError(err)

	delegatorNode.SendCovenantSigsAsValAndCheck(s.r, s.T(), s.net, s.covenantSKs, pendingDel)
	_, err = delegatorNode.WaitForNextEpoch()
	s.NoError(err)

	activeSet := delegatorNode.QueryFinalityProviderDelegations(fp.BtcPk.MarshalHex())
	s.Require().Len(activeSet, 1)
	activeDelegations, err := chain.ParseRespsBTCDelToBTCDel(activeSet[0])
	s.Require().NoError(err)
	s.Require().Len(activeDelegations.Dels, 1)
	activeDel := activeDelegations.Dels[0]
	s.Require().True(activeDel.HasCovenantQuorums(s.covenantQuorum, 0))

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(activeDel.StakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash()

	expectedSats := sdkmath.NewIntFromUint64(activeDel.TotalSat)
	s.T().Logf("tracker target before unbonding: staking_tx=%s expected_sats=%s", stakingTxHash.String(), expectedSats.String())
	var trackerReady bool
	s.Require().Eventually(func() bool {
		tracker, err := delegatorNode.QueryCostakerRewardsTracker(delegatorNode.PublicAddress)
		if err != nil {
			return false
		}
		trackerReady = tracker.ActiveSatoshis.Equal(expectedSats)
		if !trackerReady {
			s.T().Logf(
				"tracker mismatch before unbonding: staking_tx=%s tracker_sats=%s expected_sats=%s",
				stakingTxHash.String(),
				tracker.ActiveSatoshis.String(),
				expectedSats.String(),
			)
		}
		return trackerReady
	}, time.Minute, time.Second, "costaker tracker should reflect active sats before unbonding")
	s.Require().True(trackerReady)
	s.T().Logf("tracker synced before unbonding: staking_tx=%s tracker_sats=%s", stakingTxHash.String(), expectedSats.String())

	currentBtcTipResp, err := delegatorNode.QueryTip()
	s.NoError(err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	unbondingTx := activeDel.MustGetUnbondingTx()
	_, unbondingTxMsg := datagen.AddWitnessToUnbondingTx(
		s.T(),
		stakingMsgTx.TxOut[activeDel.StakingOutputIdx],
		btcDelegatorSK,
		s.covenantSKs,
		s.covenantQuorum,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		uint16(activeDel.GetStakingTime()),
		int64(activeDel.TotalSat),
		unbondingTx,
		s.net,
	)

	blockWithUnbondingTx := datagen.CreateBlockWithTransaction(s.r, currentBtcTip.Header.ToBlockHeader(), unbondingTxMsg)
	delegatorNode.InsertHeader(&blockWithUnbondingTx.HeaderBytes)
	unbondingInclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithUnbondingTx.SpvProof)

	delegatorNode.SubmitRefundableTxWithAssertion(func() {
		delegatorNode.BTCUndelegate(
			&stakingTxHash,
			unbondingTxMsg,
			unbondingInclusionProof,
			[]*wire.MsgTx{stakingMsgTx},
		)
		delegatorNode.WaitForNextBlock()
	}, true)

	s.Require().Eventually(func() bool {
		unbonded := delegatorNode.QueryUnbondedDelegations()
		for _, resp := range unbonded {
			del, err := chain.ParseRespBTCDelToBTCDel(resp)
			if err != nil {
				continue
			}
			hash := del.MustGetStakingTxHash()
			if hash.IsEqual(&stakingTxHash) {
				return true
			}
		}
		return false
	}, 2*time.Minute, time.Second, "BTC delegation should enter UNBONDED state")

	fpHex := fp.BtcPk.MarshalHex()
	s.Require().Eventually(func() bool {
		currentHeight, err := delegatorNode.QueryCurrentHeight()
		if err != nil {
			s.T().Logf("error querying height: %v", err)
			return false
		}
		activeFps := delegatorNode.QueryActiveFinalityProvidersAtHeight(uint64(currentHeight))
		for _, activeFP := range activeFps {
			if strings.EqualFold(activeFP.BtcPkHex.MarshalHex(), fpHex) {
				return false
			}
		}
		return true
	}, 2*time.Minute, 2*time.Second, "finality provider should leave active set once delegation unbonds")

	_, err = delegatorNode.WaitForNextEpoch()
	s.NoError(err)

	trackerAfter, err := delegatorNode.QueryCostakerRewardsTracker(delegatorNode.PublicAddress)
	s.NoError(err)
	s.T().Logf(
		"tracker after finality provider exit: staking_tx=%s tracker_sats=%s",
		stakingTxHash.String(),
		trackerAfter.ActiveSatoshis.String(),
	)

	s.Require().True(
		trackerAfter.ActiveSatoshis.IsZero(),
		"expected costaker to lose all sats after FP removal, but tracker still holds %s",
		trackerAfter.ActiveSatoshis.String(),
	)
}
