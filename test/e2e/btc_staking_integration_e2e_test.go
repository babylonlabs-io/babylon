package e2e

import (
	"math"

	"github.com/babylonchain/babylon/test/e2e/configurer"
	"github.com/babylonchain/babylon/test/e2e/configurer/chain"
	"github.com/babylonchain/babylon/test/e2e/initialization"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/suite"
)

type BTCStakingIntegrationTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *BTCStakingIntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var (
		err error
	)

	// The e2e test flow is as follows:
	//
	// 1. Configure 1 consumer with some validator nodes
	// 2. Execute various e2e tests
	s.configurer, err = configurer.NewBTCStakingIntegrationConfigurer(s.T(), true)

	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *BTCStakingIntegrationTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	s.Require().NoError(err)
}

func (s *BTCStakingIntegrationTestSuite) Test1RegisterNewConsumer() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	s.registerVerifyConsumer(nonValidatorNode)
}

func (s *BTCStakingIntegrationTestSuite) Test2CreateConsumerFinalityProvider() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get the consumer registered in Test1
	consumerRegistryList := nonValidatorNode.QueryConsumerRegistryList(&query.PageRequest{Limit: 1})
	consumerId := consumerRegistryList.ConsumerIds[0]

	// create a random num of finality providers from 1 to 5 on the consumer
	numFPs := datagen.RandomInt(r, 5) + 1
	for i := 0; i < int(numFPs); i++ {
		s.createVerifyConsumerFP(nonValidatorNode, consumerId)
	}
}

func (s *BTCStakingIntegrationTestSuite) Test3RestakeDelegationToMultipleFPs() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get the consumer registered in Test1
	consumerRegistryList := nonValidatorNode.QueryConsumerRegistryList(&query.PageRequest{Limit: 1})
	consumerId := consumerRegistryList.ConsumerIds[0]
	// get the consumer created in Test2
	consumerFp := nonValidatorNode.QueryConsumerFinalityProviders(consumerId)[0]

	// register a babylon finality provider
	babylonFp := s.createVerifyBabylonFP(nonValidatorNode)

	// create a delegation and restake to both Babylon and consumer finality providers
	// NOTE: this will create delegation in pending state as covenant sigs are not provided
	delBtcPk, stakingTxHash := s.createBabylonDelegation(nonValidatorNode, babylonFp, consumerFp)

	// check delegation
	delegation := nonValidatorNode.QueryBtcDelegation(stakingTxHash)
	s.NotNil(delegation)

	// check consumer finality provider delegation
	czPendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex())
	s.Len(czPendingDelSet, 1)
	czPendingDels := czPendingDelSet[0]
	s.Len(czPendingDels.Dels, 1)
	s.Equal(delBtcPk.SerializeCompressed()[1:], czPendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(czPendingDels.Dels[0].CovenantSigs, 0)

	// check Babylon finality provider delegation
	pendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(babylonFp.BtcPk.MarshalHex())
	s.Len(pendingDelSet, 1)
	pendingDels := pendingDelSet[0]
	s.Len(pendingDels.Dels, 1)
	s.Equal(delBtcPk.SerializeCompressed()[1:], pendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(pendingDels.Dels[0].CovenantSigs, 0)
}

// helper function: register a random consumer on Babylon and verify it
func (s *BTCStakingIntegrationTestSuite) registerVerifyConsumer(babylonNode *chain.NodeConfig) *bsctypes.ConsumerRegister {
	// Register a random consumer on Babylon
	randomConsumer := datagen.GenRandomConsumerRegister(r)
	babylonNode.RegisterConsumer(randomConsumer.ConsumerId, randomConsumer.ConsumerName, randomConsumer.ConsumerDescription)
	babylonNode.WaitForNextBlock()

	// Query the consumer registry to verify the consumer was registered
	consumerRegistry := babylonNode.QueryConsumerRegistry(randomConsumer.ConsumerId)
	s.Require().Len(consumerRegistry, 1)
	s.Require().Equal(randomConsumer.ConsumerId, consumerRegistry[0].ConsumerId)
	s.Require().Equal(randomConsumer.ConsumerName, consumerRegistry[0].ConsumerName)
	s.Require().Equal(randomConsumer.ConsumerDescription, consumerRegistry[0].ConsumerDescription)
	return randomConsumer
}

// helper function: create a random consumer finality provider on Babylon and verify it
func (s *BTCStakingIntegrationTestSuite) createVerifyConsumerFP(babylonNode *chain.NodeConfig, consumerId string) *bstypes.FinalityProvider {
	/*
		create a random consumer finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	czFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	czFp, err := datagen.GenRandomCustomFinalityProvider(r, czFpBTCSK, babylonNode.SecretKey, consumerId)
	s.NoError(err)
	babylonNode.CreateConsumerFinalityProvider(
		czFp.BabylonPk, czFp.BtcPk, czFp.Pop, consumerId, czFp.Description.Moniker,
		czFp.Description.Identity, czFp.Description.Website, czFp.Description.SecurityContact,
		czFp.Description.Details, czFp.Commission,
	)

	// wait for a block so that above txs take effect
	babylonNode.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFp := babylonNode.QueryConsumerFinalityProvider(consumerId, czFp.BtcPk.MarshalHex())
	s.Equal(czFp.Description, actualFp.Description)
	s.Equal(czFp.Commission, actualFp.Commission)
	s.Equal(czFp.BabylonPk, actualFp.BabylonPk)
	s.Equal(czFp.BtcPk, actualFp.BtcPk)
	s.Equal(czFp.Pop, actualFp.Pop)
	s.Equal(czFp.SlashedBabylonHeight, actualFp.SlashedBabylonHeight)
	s.Equal(czFp.SlashedBtcHeight, actualFp.SlashedBtcHeight)
	s.Equal(consumerId, actualFp.ConsumerId)
	return czFp
}

// helper function: create a random Babylon finality provider and verify it
func (s *BTCStakingIntegrationTestSuite) createVerifyBabylonFP(babylonNode *chain.NodeConfig) *bstypes.FinalityProviderResponse {
	/*
		create a random finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	babylonFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	babylonFp, err := datagen.GenRandomCustomFinalityProvider(r, babylonFpBTCSK, babylonNode.SecretKey, "")
	s.NoError(err)
	babylonNode.CreateFinalityProvider(babylonFp.BabylonPk, babylonFp.BtcPk, babylonFp.Pop, babylonFp.Description.Moniker, babylonFp.Description.Identity, babylonFp.Description.Website, babylonFp.Description.SecurityContact, babylonFp.Description.Details, babylonFp.Commission)

	// wait for a block so that above txs take effect
	babylonNode.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFps := babylonNode.QueryFinalityProviders()
	s.Len(actualFps, 1)
	s.Equal(babylonFp.Description, actualFps[0].Description)
	s.Equal(babylonFp.Commission, actualFps[0].Commission)
	s.Equal(babylonFp.BabylonPk, actualFps[0].BabylonPk)
	s.Equal(babylonFp.BtcPk, actualFps[0].BtcPk)
	s.Equal(babylonFp.Pop, actualFps[0].Pop)
	s.Equal(babylonFp.SlashedBabylonHeight, actualFps[0].SlashedBabylonHeight)
	s.Equal(babylonFp.SlashedBtcHeight, actualFps[0].SlashedBtcHeight)

	return actualFps[0]
}

// helper function: create a Babylon delegation and restake to Babylon and consumer finality providers
func (s *BTCStakingIntegrationTestSuite) createBabylonDelegation(nonValidatorNode *chain.NodeConfig, babylonFp *bstypes.FinalityProviderResponse, consumerFp *bsctypes.FinalityProviderResponse) (*btcec.PublicKey, string) {
	/*
		create a random BTC delegation restaking to Babylon and consumer finality providers
	*/

	delBbnSk := nonValidatorNode.SecretKey
	delBtcSk, delBtcPk, _ := datagen.GenRandomBTCKeyPair(r)

	// BTC staking params, BTC delegation key pairs and PoP
	params := nonValidatorNode.QueryBTCStakingParams()

	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	// get covenant BTC PKs
	covenantBTCPKs := []*btcec.PublicKey{}
	for _, covenantPK := range params.CovenantPks {
		covenantBTCPKs = append(covenantBTCPKs, covenantPK.MustToBTCPK())
	}
	// NOTE: we use the node's secret key as Babylon secret key for the BTC delegation
	pop, err := bstypes.NewPoP(delBbnSk, delBtcSk)
	s.NoError(err)
	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		s.T(),
		net,
		delBtcSk,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		params.SlashingAddress,
		params.SlashingRate,
		unbondingTime,
	)

	stakingMsgTx := testStakingInfo.StakingTx
	stakingTxHash := stakingMsgTx.TxHash().String()
	stakingSlashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		datagen.StakingOutIdx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		delBtcSk,
	)
	s.NoError(err)

	// submit staking tx to Bitcoin and get inclusion proof
	currentBtcTipResp, err := nonValidatorNode.QueryTip()
	s.NoError(err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	nonValidatorNode.InsertHeader(&blockWithStakingTx.HeaderBytes)
	// make block k-deep
	for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
		nonValidatorNode.InsertNewEmptyBtcHeader(r)
	}
	stakingTxInfo := btcctypes.NewTransactionInfoFromSpvProof(blockWithStakingTx.SpvProof)

	// generate BTC undelegation stuff
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee // TODO: parameterise fee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		s.T(),
		net,
		delBtcSk,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		params.SlashingAddress,
		params.SlashingRate,
		unbondingTime,
	)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delBtcSk)
	s.NoError(err)

	// submit the message for creating BTC delegation
	delBTCPKs := []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(delBtcPk)}
	nonValidatorNode.CreateBTCDelegation(
		delBbnSk.PubKey().(*secp256k1.PubKey),
		delBTCPKs,
		pop,
		stakingTxInfo,
		[]*bbn.BIP340PubKey{babylonFp.BtcPk, consumerFp.BtcPk},
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(unbondingTime),
		btcutil.Amount(unbondingValue),
		delUnbondingSlashingSig,
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()
	nonValidatorNode.WaitForNextBlock()

	return delBtcPk, stakingTxHash
}
