package e2e

import (
	"math"
	"math/rand"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

type BTCStakeExpansionTestSuite struct {
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

func (s *BTCStakeExpansionTestSuite) SetupSuite() {
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

func (s *BTCStakeExpansionTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestCreateFinalityProviderAndDelegation is an end-to-end test for
// user story 1: user creates finality provider, a BTC delegation and
// a BTC expansion delegation
func (s *BTCStakeExpansionTestSuite) Test1CreateStakeExpansionDelegation() {
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

	// create a BTC expansion delegation
	stkExpDelStakingSlashingInfo, prevDelStakingInfo, fundingTx := nonValidatorNode.CreateBTCDelegationWithExpansionAndCheck(
		s.r,
		s.T(),
		s.net,
		nonValidatorNode.WalletName,
		[]*bstypes.FinalityProvider{s.cacheFP},
		s.delBTCSK,
		nonValidatorNode.PublicAddress,
		stakingTimeBlocks,
		s.stakingValue,
		s.covenantSKs,
		s.covenantQuorum,
	)

	// check stake expansion delegation is pending
	stkExpTxHash := stkExpDelStakingSlashingInfo.StakingTx.TxHash()
	stkExpDelegation := nonValidatorNode.QueryBtcDelegation(stkExpTxHash.String())
	s.NotNil(stkExpDelegation)
	s.Equal(stkExpDelegation.BtcDelegation.StakerAddr, nonValidatorNode.PublicAddress)
	s.NotNil(stkExpDelegation.BtcDelegation.StkExp)
	s.Equal(stkExpDelegation.BtcDelegation.StatusDesc, bstypes.BTCDelegationStatus_PENDING.String())

	// Step 4: submit covenant signature to verify the BTC expansion delegation
	stkExpDel, err := chain.ParseRespBTCDelToBTCDel(stkExpDelegation.BtcDelegation)
	s.NoError(err)
	nonValidatorNode.SendCovenantSigsAsValAndCheck(s.r, s.T(), s.net, s.covenantSKs, stkExpDel)

	// ensure the BTC staking expansion delegation is verified now
	stkExpDelegation = nonValidatorNode.QueryBtcDelegation(stkExpTxHash.String())
	s.NotNil(stkExpDelegation)
	s.Equal(stkExpDelegation.BtcDelegation.StatusDesc, bstypes.BTCDelegationStatus_VERIFIED.String())

	activeDelRes := nonValidatorNode.QueryBtcDelegation(prevDelStakingInfo.StakingTx.TxHash().String())
	s.NotNil(activeDelRes)
	activeDel, err := chain.ParseRespBTCDelToBTCDel(activeDelRes.BtcDelegation)
	s.NoError(err)

	// Step 5: submit MsgBTCUndelegate for the original BTC delegation
	// to activate the BTC expansion delegation
	// spendingTx of the previous BTC delegation
	// staking output is the staking tx of the BTC stake expansion delegation
	spendingTx := stkExpDelStakingSlashingInfo.StakingTx

	_, stakeExpTxMsg := datagen.AddWitnessToStakeExpTx(
		s.T(),
		prevDelStakingInfo.StakingTx.TxOut[activeDel.StakingOutputIdx],
		fundingTx.TxOut[0],
		s.delBTCSK,
		s.covenantSKs,
		s.covenantQuorum,
		[]*btcec.PublicKey{s.cacheFP.BtcPk.MustToBTCPK()},
		uint16(activeDel.GetStakingTime()),
		int64(activeDel.TotalSat),
		spendingTx,
		s.net,
	)

	currentBtcTipResp, err := nonValidatorNode.QueryTip()
	s.NoError(err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	blockWithUnbondingTx := datagen.CreateBlockWithTransaction(s.r, currentBtcTip.Header.ToBlockHeader(), stakeExpTxMsg)
	nonValidatorNode.InsertHeader(&blockWithUnbondingTx.HeaderBytes)
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithUnbondingTx.SpvProof)

	// wait for stake expansion transaction to be k-deep
	for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
		nonValidatorNode.InsertNewEmptyBtcHeader(s.r)
	}

	prevDelStakingTxHash := prevDelStakingInfo.StakingTx.TxHash()
	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		// submit the message for creating BTC undelegation
		txHash := nonValidatorNode.BTCUndelegate(
			&prevDelStakingTxHash,
			stakeExpTxMsg,
			inclusionProof,
			[]*wire.MsgTx{
				prevDelStakingInfo.StakingTx,
				fundingTx,
			},
		)
		// wait for a block so that above txs take effect
		nonValidatorNode.WaitForNextBlocks(2)

		res, _ := nonValidatorNode.QueryTx(txHash)
		s.Equal(res.Code, uint32(0), res.RawLog)
	}, true)

	// Wait for unbonded delegations to be created
	var unbondedDelsResp []*bstypes.BTCDelegationResponse
	s.Eventually(func() bool {
		unbondedDelsResp = nonValidatorNode.QueryUnbondedDelegations()
		return len(unbondedDelsResp) > 0
	}, time.Minute, time.Second*2)

	unbondDel, err := chain.ParseRespBTCDelToBTCDel(unbondedDelsResp[0])
	s.NoError(err)
	s.Equal(prevDelStakingTxHash, unbondDel.MustGetStakingTxHash())

	// ensure the BTC staking expansion delegation is active now
	stkExpDelegation = nonValidatorNode.QueryBtcDelegation(stkExpDelStakingSlashingInfo.StakingTx.TxHash().String())
	s.NotNil(stkExpDelegation)
	s.Equal(stkExpDelegation.BtcDelegation.StatusDesc, bstypes.BTCDelegationStatus_ACTIVE.String())
}
