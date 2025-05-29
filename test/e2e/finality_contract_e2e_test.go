package e2e

import (
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
)

const (
	ConsumerID = "optimism-1234"
)

type FinalityContractTestSuite struct {
	suite.Suite

	r              *rand.Rand
	net            *chaincfg.Params
	delBtcSk       *btcec.PrivateKey
	babylonFp      *bstypes.FinalityProvider
	consumerBtcSk  *btcec.PrivateKey
	consumerFp     *bstypes.FinalityProvider
	covenantSks    []*btcec.PrivateKey
	covenantQuorum uint32
	stakingValue   int64
	configurer     configurer.Configurer

	// Cross-test config data
	finalityContractAddr string
}

func (s *FinalityContractTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.delBtcSk, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	covenantSks, _, covenantQuorum := bstypes.DefaultCovenantCommittee()
	s.covenantSks = covenantSks
	s.covenantQuorum = covenantQuorum
	s.stakingValue = int64(2 * 10e8)

	// The e2e test flow is as follows:
	//
	// 1. Configure a chain - chain A.
	//   * Initialize configs and genesis.
	// 2. Start network.
	// 3. Execute various e2e tests.
	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *FinalityContractTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *FinalityContractTestSuite) Test1InstantiateFinalityContract() {
	// Wait for the chain to start
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)

	contractPath := "/bytecode/op_finality_gadget.wasm"
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// Store the wasm code
	latestWasmId := int(nonValidatorNode.QueryLatestWasmCodeID())
	nonValidatorNode.StoreWasmCode(contractPath, initialization.ValidatorWalletName)
	s.Eventually(func() bool {
		newLatestWasmId := int(nonValidatorNode.QueryLatestWasmCodeID())
		if latestWasmId+1 > newLatestWasmId {
			return false
		}
		latestWasmId = newLatestWasmId
		return true
	}, time.Second*20, time.Second)

	// Instantiate the finality gadget contract
	adminAddr := "bbn1gl0ctnctxr43npuyswfq5wz67r8p5kmsu0xhmy"
	nonValidatorNode.InstantiateWasmContract(
		strconv.Itoa(latestWasmId),
		`{
			"admin": "`+adminAddr+`",
			"consumer_id": "`+ConsumerID+`",
			"is_enabled": true
		}`,
		initialization.ValidatorWalletName,
	)

	var contracts []string
	s.Eventually(func() bool {
		contracts, err = nonValidatorNode.QueryContractsFromId(latestWasmId)
		return err == nil && len(contracts) == 1
	}, time.Second*10, time.Second)
	s.finalityContractAddr = contracts[0]
	s.T().Log("Finality gadget contract address: ", s.finalityContractAddr)
}

func (s *FinalityContractTestSuite) Test2RegisterRollupConsumer() {
	var registeredConsumer *bsctypes.ConsumerRegister
	var err error

	// Register the consumer id on Babylon
	registeredConsumer = bsctypes.NewCosmosConsumerRegister(
		ConsumerID,
		datagen.GenRandomHexStr(s.r, 5),
		"Chain description: "+datagen.GenRandomHexStr(s.r, 15),
		3,
	)

	validatorNode, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(0)
	require.NoError(s.T(), err)

	// TODO: Register the Consumer through a gov proposal
	validatorNode.RegisterRollupConsumerChain(initialization.ValidatorWalletName, registeredConsumer.ConsumerId, registeredConsumer.ConsumerName, registeredConsumer.ConsumerDescription, s.finalityContractAddr, 3)

	nonValidatorNode, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	require.NoError(s.T(), err)

	// Confirm the consumer is registered
	s.Eventually(func() bool {
		consumerRegistryResp := nonValidatorNode.QueryBTCStkConsumerConsumer(ConsumerID)
		s.Require().NotNil(consumerRegistryResp)
		s.Require().Len(consumerRegistryResp.ConsumerRegisters, 1)
		s.Require().Equal(registeredConsumer.ConsumerId, consumerRegistryResp.ConsumerRegisters[0].ConsumerId)
		s.Require().Equal(registeredConsumer.ConsumerName, consumerRegistryResp.ConsumerRegisters[0].ConsumerName)
		s.Require().Equal(registeredConsumer.ConsumerDescription, consumerRegistryResp.ConsumerRegisters[0].ConsumerDescription)

		return true
	}, 10*time.Second, 2*time.Second, "Consumer was not registered within the expected time")

	s.T().Logf("Consumer registered: ID=%s, Name=%s, Description=%s",
		registeredConsumer.ConsumerId,
		registeredConsumer.ConsumerName,
		registeredConsumer.ConsumerDescription)
}

func (s *FinalityContractTestSuite) Test3CreateConsumerFPAndDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	// Create and register a Babylon FP first
	validatorNode, err := chainA.GetNodeAtIndex(0)
	require.NoError(s.T(), err)

	babylonFpSk, _, err := datagen.GenRandomBTCKeyPair(s.r)
	require.NoError(s.T(), err)

	s.babylonFp = chain.CreateFpFromNodeAddr(
		s.T(),
		s.r,
		babylonFpSk,
		validatorNode,
	)
	s.Require().NotNil(s.babylonFp)

	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	require.NoError(s.T(), err)
	// Create and register a Consumer FP next
	s.consumerBtcSk, _, err = datagen.GenRandomBTCKeyPair(s.r)
	s.Require().NoError(err)

	s.consumerFp = chain.CreateConsumerFpFromNodeAddr(
		s.T(),
		s.r,
		ConsumerID,
		s.consumerBtcSk,
		nonValidatorNode,
	)
	s.Require().NotNil(s.consumerFp)

	/*
		create a random BTC delegation under these finality providers
	*/

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)

	// NOTE: we use the node's address for the BTC delegation
	testStakingInfo := nonValidatorNode.CreateBTCDelegationMultipleFPsAndCheck(
		s.r,
		s.T(),
		s.net,
		nonValidatorNode.WalletName,
		[]*bstypes.FinalityProvider{
			s.babylonFp,
			s.consumerFp,
		},
		s.delBtcSk,
		nonValidatorNode.PublicAddress,
		stakingTimeBlocks,
		s.stakingValue,
	)

	// Check babylon delegation
	pendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(s.babylonFp.BtcPk.MarshalHex())
	s.Len(pendingDelSet, 1)
	pendingDels := pendingDelSet[0]
	s.Len(pendingDels.Dels, 1)
	s.Equal(s.delBtcSk.PubKey().SerializeCompressed()[1:], pendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(pendingDels.Dels[0].CovenantSigs, 0)

	// check delegation
	delegation := nonValidatorNode.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.NotNil(delegation)
	s.Equal(delegation.BtcDelegation.StakerAddr, nonValidatorNode.PublicAddress)
}

func (s *FinalityContractTestSuite) Test4SubmitCovenantSignature() {
	chainA := s.configurer.GetChainConfig(0)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get the last BTC delegation
	pendingDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(s.babylonFp.BtcPk.MarshalHex())
	s.Len(pendingDelsSet, 1)
	pendingDels := pendingDelsSet[0]
	s.Len(pendingDels.Dels, 1)
	pendingDelResp := pendingDels.Dels[0]
	pendingDel, err := chain.ParseRespBTCDelToBTCDel(pendingDelResp)
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
		generate and insert new covenant signature, to activate the BTC delegation
	*/
	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		s.covenantSks,
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
		s.covenantSks,
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
		s.covenantSks,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	s.NoError(err)

	for i := 0; i < int(s.covenantQuorum); i++ {
		// add covenant sigs
		nonValidatorNode.AddCovenantSigsFromVal(
			covenantSlashingSigs[i].CovPk,
			stakingTxHash,
			covenantSlashingSigs[i].AdaptorSigs,
			bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
			covenantUnbondingSlashingSigs[i].AdaptorSigs,
		)
		nonValidatorNode.WaitForNextBlock()
	}

	// Ensure the BTC delegation has covenant sigs now
	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(s.consumerFp.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)

	activeDels, err := chain.ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(s.covenantQuorum))
}

func (s *FinalityContractTestSuite) Test5CommitPublicRandomnessAndSubmitFinalitySignature() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	/*
		commit some amount of public randomness
	*/
	// commit public randomness list
	numPubRand := uint64(100)
	commitStartHeight := uint64(1)
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.consumerBtcSk, commitStartHeight, numPubRand)
	s.NoError(err)

	nonValidatorNode.CommitPubRandListConsumer(
		ConsumerID,
		msgCommitPubRandList.FpBtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)

	// Query the public randomness commitment from the finality contract
	s.Eventually(func() bool {
		commitment, err := nonValidatorNode.QueryPublicRandomnessCommitment(ConsumerID, commitStartHeight)
		if err != nil {
			return false
		}
		if commitment == nil {
			return false
		}
		s.Equal(randListInfo.Commitment, commitment.Commitment)
		s.Equal(msgCommitPubRandList.NumPubRand, commitment.NumPubRand)
		s.Equal(msgCommitPubRandList.FpBtcPk.MarshalHex(), commitment.FpBtcPk.MarshalHex())
		return true
	}, time.Millisecond*500, time.Second*10, "Public randomness commitment was not found within the expected time")

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
	sig, err := eots.Sign(s.consumerBtcSk, randListInfo.SRList[idx], msgToSign)
	s.NoError(err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)

	nonValidatorNode.SubmitRefundableTxWithAssertion(func() {
		// submit finality signature
		nonValidatorNode.AddFinalitySigFromVal(s.consumerFp.BtcPk, activatedHeight, &randListInfo.PRList[idx], *randListInfo.ProofList[idx].ToProto(), appHash, eotsSig)

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
}
