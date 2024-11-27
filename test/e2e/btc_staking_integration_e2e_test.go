package e2e

import (
	"encoding/hex"
	"math"
	"math/rand"
	"time"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/stretchr/testify/suite"
)

var (
	r   = rand.New(rand.NewSource(time.Now().Unix()))
	net = &chaincfg.SimNetParams
	// BTC delegation
	delBTCSK, delBTCPK, _ = datagen.GenRandomBTCKeyPair(r)
	// covenant
	covenantSKs, _, covenantQuorum = bstypes.DefaultCovenantCommittee()

	stakingValue = int64(2 * 10e8)
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

// Test1RegisterNewConsumer registers a new consumer on Babylon
func (s *BTCStakingIntegrationTestSuite) Test1RegisterNewConsumer() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	babylonNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	consumerID := s.getIBCClientID()
	s.registerVerifyConsumer(babylonNode, consumerID)
}

// Test2CreateConsumerFinalityProvider -
// 1. Creates a consumer finality provider under the consumer registered in Test1RegisterNewConsumer
// 2. Verifies that the finality provider is stored in the smart contract
func (s *BTCStakingIntegrationTestSuite) Test2CreateConsumerFinalityProvider() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// retrieve the registered consumer in Test1
	consumerRegistryList := nonValidatorNode.QueryConsumerRegistryList(&query.PageRequest{Limit: 1})
	s.NotNil(consumerRegistryList)
	s.NotNil(consumerRegistryList.ConsumerIds)
	s.Equal(1, len(consumerRegistryList.ConsumerIds))
	consumerId := consumerRegistryList.ConsumerIds[0]

	// generate a random number of finality providers from 1 to 5
	numConsumerFPs := datagen.RandomInt(r, 5) + 1
	var consumerFps []*bstypes.FinalityProvider
	for i := 0; i < int(numConsumerFPs); i++ {
		consumerFp := s.createVerifyConsumerFP(nonValidatorNode, consumerId)
		consumerFps = append(consumerFps, consumerFp)
	}

	czNode, err := s.configurer.GetChainConfig(1).GetNodeAtIndex(2)
	s.NoError(err)
	// retrieve staking contract address and query finality providers stored in the contract
	stakingContracts, err := czNode.QueryContractsFromId(2)
	s.NoError(err)
	s.Equal(1, len(stakingContracts))
	stakingContractAddr := stakingContracts[0]

	// query the staking contract for finality providers
	var dataFromContract *chain.ConsumerFpsResponse
	s.Eventually(func() bool {
		// try to retrieve expected number of finality providers from the contract
		dataFromContract, err = czNode.QueryConsumerFps(stakingContractAddr)
		if err != nil {
			return false
		}
		return len(dataFromContract.ConsumerFps) == int(numConsumerFPs)
	}, time.Second*20, time.Second)

	// create a map of expected finality providers for verification
	fpMap := make(map[string]*bstypes.FinalityProvider)
	for _, czFp := range consumerFps {
		fpMap[czFp.BtcPk.MarshalHex()] = czFp
	}

	// validate that all finality providers match with the consumer list
	for _, czFp := range dataFromContract.ConsumerFps {
		fpFromMap, ok := fpMap[czFp.BtcPkHex]
		s.True(ok)
		s.Equal(fpFromMap.BtcPk.MarshalHex(), czFp.BtcPkHex)
		s.Equal(fpFromMap.SlashedBabylonHeight, czFp.SlashedHeight)
		s.Equal(fpFromMap.SlashedBtcHeight, czFp.SlashedBtcHeight)
		s.Equal(fpFromMap.ConsumerId, czFp.ConsumerID)
	}
}

// Test3RestakeDelegationToMultipleFPs creates a Babylon delegation and restakes
// it to both Babylon and consumer finality provider created in Test2CreateConsumerFinalityProvider
// This will create a delegation in pending state as covenant sigs are not provided
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

// Test4ActivateDelegation -
// 1. Activates the delegation created in Test3RestakeDelegationToMultipleFPs by submitting covenant sigs
// 2. Verifies that the active delegation is stored in the smart contract
func (s *BTCStakingIntegrationTestSuite) Test4ActivateDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	consumerRegistryList := nonValidatorNode.QueryConsumerRegistryList(&query.PageRequest{Limit: 1})
	consumerId := consumerRegistryList.ConsumerIds[0]
	// get the consumer created in Test2
	consumerFp := nonValidatorNode.QueryConsumerFinalityProviders(consumerId)[0]

	// activate the delegation created in Test3 by submitting covenant sigs
	s.submitCovenantSigs(nonValidatorNode, consumerFp)

	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	czNode, err := s.configurer.GetChainConfig(1).GetNodeAtIndex(2)
	s.NoError(err)
	stakingContracts, err := czNode.QueryContractsFromId(2)
	s.NoError(err)
	stakingContractAddr := stakingContracts[0]

	// query the staking contract for delegations
	var data1FromContract *chain.ConsumerDelegationsResponse
	s.Eventually(func() bool {
		// try to retrieve expected number of delegations from the contract
		data1FromContract, err = czNode.QueryConsumerDelegations(stakingContractAddr)
		if err != nil {
			return false
		}
		return len(data1FromContract.ConsumerDelegations) == 1
	}, time.Second*20, time.Second)

	s.Empty(data1FromContract.ConsumerDelegations[0].UndelegationInfo.DelegatorUnbondingSig) // assert there is no delegator unbonding sig
	s.Equal(activeDel.BtcPk.MarshalHex(), data1FromContract.ConsumerDelegations[0].BtcPkHex)
	s.Len(data1FromContract.ConsumerDelegations[0].FpBtcPkList, 2)
	s.Equal(activeDel.FpBtcPkList[0].MarshalHex(), data1FromContract.ConsumerDelegations[0].FpBtcPkList[0])
	s.Equal(activeDel.FpBtcPkList[1].MarshalHex(), data1FromContract.ConsumerDelegations[0].FpBtcPkList[1])
	s.Equal(activeDel.StartHeight, data1FromContract.ConsumerDelegations[0].StartHeight)
	s.Equal(activeDel.EndHeight, data1FromContract.ConsumerDelegations[0].EndHeight)
	s.Equal(activeDel.TotalSat, data1FromContract.ConsumerDelegations[0].TotalSat)
	s.Equal(hex.EncodeToString(activeDel.StakingTx), hex.EncodeToString(data1FromContract.ConsumerDelegations[0].StakingTx))
	s.Equal(activeDel.SlashingTx.ToHexStr(), hex.EncodeToString(data1FromContract.ConsumerDelegations[0].SlashingTx))

	// assert the fp voting power is updated in the staking contract
	var data2FromContract *chain.ConsumerFpsByPowerResponse
	fpsWithPower := make([]chain.ConsumerFpInfo, 0)
	s.Eventually(func() bool {
		data2FromContract, err = czNode.QueryConsumerFpsByPower(stakingContractAddr)
		if err != nil {
			return false
		}
		// Filter out no power fps
		for _, fp := range data2FromContract.Fps {
			if fp.Power > 0 {
				fpsWithPower = append(fpsWithPower, fp)
			}
		}

		return len(fpsWithPower) == 1
	}, time.Second*20, time.Second)

	totalPower := uint64(0)
	for _, fp := range data2FromContract.Fps {
		totalPower += fp.Power
	}

	czPowerFromNode := s.getFpTotalPowerFromBabylonNode(consumerFp)
	s.Equal(czPowerFromNode, totalPower)
}

// Test5UnbondDelegation -
// 1. Unbond the delegation created in Test3RestakeDelegationToMultipleFPs
// 2. Verify that the unbonded delegation is stored in the smart contract
func (s *BTCStakingIntegrationTestSuite) Test5UnbondDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	consumerRegistryList := nonValidatorNode.QueryConsumerRegistryList(&query.PageRequest{Limit: 1})
	consumerId := consumerRegistryList.ConsumerIds[0]
	// get the consumer created in Test2
	consumerFp := nonValidatorNode.QueryConsumerFinalityProviders(consumerId)[0]

	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex())
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

	// get unbonding tx
	unbondingTx := activeDel.BtcUndelegation.UnbondingTx
	unbondingTxMsg, err := bbn.NewBTCTxFromBytes(unbondingTx)
	s.NoError(err)
	// get inclusion proof of the unbonding tx
	currentBtcTipResp, err := nonValidatorNode.QueryTip()
	s.NoError(err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)
	blockWithUnbondingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), unbondingTxMsg)
	nonValidatorNode.InsertHeader(&blockWithUnbondingTx.HeaderBytes)
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithUnbondingTx.SpvProof)
	// submit the message for creating BTC undelegation
	nonValidatorNode.BTCUndelegate(&stakingTxHash, unbondingTxMsg, inclusionProof)
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
	stakingTxHashHex := unbondDel.MustGetStakingTxHash().String()

	czNode, err := s.configurer.GetChainConfig(1).GetNodeAtIndex(2)
	s.NoError(err)

	stakingContracts, err := czNode.QueryContractsFromId(2)
	s.NoError(err)
	stakingContractAddr := stakingContracts[0]

	contractDel, err := czNode.QuerySingleConsumerDelegation(stakingContractAddr, stakingTxHashHex)
	s.NoError(err)
	s.NotNil(contractDel.UndelegationInfo.DelegatorUnbondingSig) // assert delegator unbonding sig exists
	s.Equal(unbondDel.BtcPk.MarshalHex(), contractDel.BtcPkHex)
	s.Len(contractDel.FpBtcPkList, 2)
	s.Equal(unbondDel.FpBtcPkList[0].MarshalHex(), contractDel.FpBtcPkList[0])
	s.Equal(unbondDel.FpBtcPkList[1].MarshalHex(), contractDel.FpBtcPkList[1])
	s.Equal(unbondDel.StartHeight, contractDel.StartHeight)
	s.Equal(unbondDel.EndHeight, contractDel.EndHeight)
	s.Equal(unbondDel.TotalSat, contractDel.TotalSat)
	s.Equal(hex.EncodeToString(unbondDel.StakingTx), hex.EncodeToString(contractDel.StakingTx))
	s.Equal(unbondDel.SlashingTx.ToHexStr(), hex.EncodeToString(contractDel.SlashingTx))

	// assert the fp voting power is updated in the staking contract
	var dataFromContract *chain.ConsumerFpsByPowerResponse
	s.Eventually(func() bool {
		dataFromContract, err = czNode.QueryConsumerFpsByPower(stakingContractAddr)
		if err != nil {
			return false
		}
		return len(dataFromContract.Fps) >= 1
	}, time.Second*20, time.Second)

	totalPower := uint64(0)
	for _, fp := range dataFromContract.Fps {
		totalPower += fp.Power
	}

	s.Equal(uint64(0), totalPower) // expect the power to be 0 after unbonding
}

// Test6ContractQueries -
// 1. Query all finality providers stored in the staking contract
// 2. Query all BTC delegations stored in the staking contract
// 3. Query a single finality provider from the staking contract
// 4. Query a single BTC delegation from the staking contract
// 5. Query BTC delegations of a specific finality provider from the staking contract
func (s *BTCStakingIntegrationTestSuite) Test6ContractQueries() {
	// 1. Already covered in Test2CreateConsumerFinalityProvider
	// 2. Already covered in Test4ActivateDelegation

	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	consumerRegistryList := nonValidatorNode.QueryConsumerRegistryList(&query.PageRequest{Limit: 1})
	consumerId := consumerRegistryList.ConsumerIds[0]
	consumerFp := nonValidatorNode.QueryConsumerFinalityProviders(consumerId)[0]

	czNode, err := s.configurer.GetChainConfig(1).GetNodeAtIndex(2)
	s.NoError(err)

	stakingContracts, err := czNode.QueryContractsFromId(2)
	s.NoError(err)
	stakingContractAddr := stakingContracts[0]

	// 3. Query a single finality provider from the staking contract
	contractFP, err := czNode.QuerySingleConsumerFp(stakingContractAddr, consumerFp.BtcPk.MarshalHex())
	s.NoError(err)
	s.Equal(consumerFp.BtcPk.MarshalHex(), contractFP.BtcPkHex)
	s.Equal(consumerFp.SlashedBabylonHeight, contractFP.SlashedHeight)
	s.Equal(consumerFp.SlashedBtcHeight, contractFP.SlashedBtcHeight)
	s.Equal(consumerFp.ConsumerId, contractFP.ConsumerID)

	// 4. Query a single BTC delegation from the staking contract
	nodeFpDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex())
	s.Len(nodeFpDelsSet, 1)
	nodeFpDels, err := ParseRespsBTCDelToBTCDel(nodeFpDelsSet[0])
	s.NoError(err)
	s.NotNil(nodeFpDels)
	s.Len(nodeFpDels.Dels, 1)
	nodefpDel := nodeFpDels.Dels[0]
	stakingTxHashHex := nodefpDel.MustGetStakingTxHash().String()
	contractDel, err := czNode.QuerySingleConsumerDelegation(stakingContractAddr, stakingTxHashHex)
	s.NoError(err)
	s.Equal(nodefpDel.BtcPk.MarshalHex(), contractDel.BtcPkHex)
	s.Len(contractDel.FpBtcPkList, 2)
	s.Equal(nodefpDel.FpBtcPkList[0].MarshalHex(), contractDel.FpBtcPkList[0])
	s.Equal(nodefpDel.FpBtcPkList[1].MarshalHex(), contractDel.FpBtcPkList[1])
	s.Equal(nodefpDel.StartHeight, contractDel.StartHeight)
	s.Equal(nodefpDel.EndHeight, contractDel.EndHeight)
	s.Equal(nodefpDel.TotalSat, contractDel.TotalSat)
	s.Equal(hex.EncodeToString(nodefpDel.StakingTx), hex.EncodeToString(contractDel.StakingTx))
	s.Equal(nodefpDel.SlashingTx.ToHexStr(), hex.EncodeToString(contractDel.SlashingTx))

	// 5. Query BTC delegations of a specific finality provider from the staking contract
	contractDelsByFp, err := czNode.QueryConsumerDelegationsByFp(stakingContractAddr, consumerFp.BtcPk.MarshalHex())
	s.NoError(err)
	s.NotNil(contractDelsByFp)
	s.Len(contractDelsByFp.StakingTxHashes, 1)
	s.Equal(nodefpDel.MustGetStakingTxHash().String(), contractDelsByFp.StakingTxHashes[0])
}

// TODO: add test for slashing when its supported in smart contract

// helper function: register a random consumer on Babylon and verify it
func (s *BTCStakingIntegrationTestSuite) registerVerifyConsumer(babylonNode *chain.NodeConfig, consumerID string) *bsctypes.ConsumerRegister {
	// Register a random consumer on Babylon
	randomConsumer := bsctypes.NewCosmosConsumerRegister(
		consumerID,
		datagen.GenRandomHexStr(r, 5),
		"Chain description: "+datagen.GenRandomHexStr(r, 15),
	)
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
func (s *BTCStakingIntegrationTestSuite) createVerifyConsumerFP(nonValidatorNode *chain.NodeConfig, consumerId string) *bstypes.FinalityProvider {
	/*
		create a random consumer finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	czFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	fpBabylonAddr, err := sdk.AccAddressFromBech32(nonValidatorNode.PublicAddress)
	s.NoError(err)
	czFp, err := datagen.GenCustomFinalityProvider(r, czFpBTCSK, fpBabylonAddr, consumerId)

	s.NoError(err)
	nonValidatorNode.CreateConsumerFinalityProvider(
		"val",
		czFp.BtcPk, czFp.Pop, consumerId, czFp.Description.Moniker,
		czFp.Description.Identity, czFp.Description.Website, czFp.Description.SecurityContact,
		czFp.Description.Details, czFp.Commission,
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFp := nonValidatorNode.QueryConsumerFinalityProvider(consumerId, czFp.BtcPk.MarshalHex())
	s.Equal(czFp.Description, actualFp.Description)
	s.Equal(czFp.Commission, actualFp.Commission)
	s.Equal(czFp.BtcPk, actualFp.BtcPk)
	s.Equal(czFp.Pop, actualFp.Pop)
	s.Equal(czFp.SlashedBabylonHeight, actualFp.SlashedBabylonHeight)
	s.Equal(czFp.SlashedBtcHeight, actualFp.SlashedBtcHeight)
	s.Equal(consumerId, actualFp.ConsumerId)
	return czFp
}

// helper function: create a random Babylon finality provider and verify it
func (s *BTCStakingIntegrationTestSuite) createVerifyBabylonFP(nonValidatorNode *chain.NodeConfig) *bstypes.FinalityProviderResponse {

	/*
		create a random finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	babylonFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	fpBabylonAddr, err := sdk.AccAddressFromBech32(nonValidatorNode.PublicAddress)
	s.NoError(err)
	babylonFp, err := datagen.GenCustomFinalityProvider(r, babylonFpBTCSK, fpBabylonAddr, "")
	s.NoError(err)
	nonValidatorNode.CreateFinalityProvider("val", babylonFp.BtcPk, babylonFp.Pop, babylonFp.Description.Moniker, babylonFp.Description.Identity, babylonFp.Description.Website, babylonFp.Description.SecurityContact, babylonFp.Description.Details, babylonFp.Commission)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFps := nonValidatorNode.QueryFinalityProviders()
	s.Len(actualFps, 1)
	s.Equal(babylonFp.Description, actualFps[0].Description)
	s.Equal(babylonFp.Commission, actualFps[0].Commission)
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

	delBabylonAddr, err := sdk.AccAddressFromBech32(nonValidatorNode.PublicAddress)
	s.NoError(err)
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
	pop, err := bstypes.NewPoPBTC(delBabylonAddr, delBTCSK)
	s.NoError(err)
	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		s.T(),
		net,
		delBTCSK,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	stakingMsgTx := testStakingInfo.StakingTx
	stakingMsgTxBytes, err := bbn.SerializeBTCTx(stakingMsgTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash().String()
	stakingSlashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		datagen.StakingOutIdx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		delBTCSK,
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
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithStakingTx.SpvProof)

	// generate BTC undelegation stuff
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee // TODO: parameterise fee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		s.T(),
		net,
		delBTCSK,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delBTCSK)
	s.NoError(err)

	// submit the message for creating BTC delegation
	delBtcPK := bbn.NewBIP340PubKeyFromBTCPK(delBTCPK)
	nonValidatorNode.CreateBTCDelegation(
		*delBtcPK,
		pop,
		stakingMsgTxBytes,
		inclusionProof,
		[]*bbn.BIP340PubKey{babylonFp.BtcPk, consumerFp.BtcPk},
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		unbondingTime,
		btcutil.Amount(unbondingValue),
		delUnbondingSlashingSig,
		"val",
		false,
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()
	nonValidatorNode.WaitForNextBlock()

	return delBTCPK, stakingTxHash
}

// helper function: verify if the ibc channels are open and get the ibc client id of the CZ node
func (s *BTCStakingIntegrationTestSuite) getIBCClientID() string {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	babylonNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	chainB := s.configurer.GetChainConfig(1)
	chainB.WaitUntilHeight(1)
	czNode, err := chainB.GetNodeAtIndex(2)
	s.NoError(err)

	var babylonChannel *channeltypes.IdentifiedChannel
	s.Eventually(func() bool {
		babylonChannelsResp, err := babylonNode.QueryIBCChannels()
		if err != nil {
			return false
		}
		if len(babylonChannelsResp.Channels) != 1 {
			return false
		}
		// channel has to be open and ordered
		babylonChannel = babylonChannelsResp.Channels[0]
		if babylonChannel.State != channeltypes.OPEN {
			return false
		}
		s.Equal(channeltypes.ORDERED, babylonChannel.Ordering)
		// the counterparty has to be the Babylon smart contract
		s.Contains(babylonChannel.Counterparty.PortId, "wasm.")
		return true
	}, time.Minute, time.Second*2)

	// Wait until the channel (CZ side) is open
	var czChannel *channeltypes.IdentifiedChannel
	s.Eventually(func() bool {
		czChannelsResp, err := czNode.QueryIBCChannels()
		if err != nil {
			return false
		}
		if len(czChannelsResp.Channels) != 1 {
			return false
		}
		czChannel = czChannelsResp.Channels[0]
		if czChannel.State != channeltypes.OPEN {
			return false
		}
		s.Equal(channeltypes.ORDERED, czChannel.Ordering)
		s.Equal(babylonChannel.PortId, czChannel.Counterparty.PortId)
		return true
	}, time.Minute, time.Second*2)

	czChannelState, err := czNode.QueryChannelClientState(czChannel.ChannelId, czChannel.PortId)
	s.NoError(err)

	nextSequenceRecv, err := czNode.QueryNextSequenceReceive(babylonChannel.Counterparty.ChannelId, babylonChannel.Counterparty.PortId)
	s.NoError(err)
	// there are no packets sent yet, so the next sequence receive should be 1
	s.Equal(uint64(1), nextSequenceRecv.NextSequenceReceive)
	return czChannelState.IdentifiedClientState.GetClientId()
}

// helper function: submit covenant sigs to activate the BTC delegation
func (s *BTCStakingIntegrationTestSuite) submitCovenantSigs(nonValidatorNode *chain.NodeConfig, consumerFp *bsctypes.FinalityProviderResponse) {
	pendingDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex())
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
	activeDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(covenantQuorum))

	// wait for a block so that above txs take effect and the voting power table
	// is updated in the next block's BeginBlock
	nonValidatorNode.WaitForNextBlock()

	// ensure BTC staking is activated
	activatedHeight, err := nonValidatorNode.QueryActivatedHeight()
	s.NoError(err)
	s.Positive(activatedHeight)
	// ensure finality provider has voting power at activated height
	currentBtcTip, err := nonValidatorNode.QueryTip()
	s.NoError(err)
	activeFps := nonValidatorNode.QueryActiveFinalityProvidersAtHeight(activatedHeight)
	s.Len(activeFps, 1)
	s.Equal(activeFps[0].VotingPower, activeDels.VotingPower(currentBtcTip.Height, initialization.BabylonBtcFinalizationPeriod, params.CovenantQuorum))
}

// helper function: calculate the total voting power of a given consumer finality provider
func (s *BTCStakingIntegrationTestSuite) getFpTotalPowerFromBabylonNode(consumerFp *bsctypes.FinalityProviderResponse) uint64 {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get all delegations of the consumer finality provider
	czDelsSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex())
	s.Len(czDelsSet, 1)

	return czDelsSet[0].Dels[0].TotalSat
}
