package e2e

import (
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"

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

	r            *rand.Rand
	net          *chaincfg.Params
	delBTCSK     *btcec.PrivateKey
	stakingValue int64
	configurer   configurer.Configurer

	feePayerAddr string

	// Cross-test config data
	FinalityContractAddr string
}

func (s *FinalityContractTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.delBTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
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
	s.FinalityContractAddr = contracts[0]
	s.T().Log("Finality gadget contract address: ", s.FinalityContractAddr)
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
	validatorNode.RegisterRollupConsumerChain(initialization.ValidatorWalletName, registeredConsumer.ConsumerId, registeredConsumer.ConsumerName, registeredConsumer.ConsumerDescription, s.FinalityContractAddr, 3)

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

	babylonFpSk, _, err := datagen.GenRandomBTCKeyPair(s.r)

	babylonFp := chain.CreateFpFromNodeAddr(
		s.T(),
		s.r,
		babylonFpSk,
		validatorNode,
	)
	s.Require().NotNil(babylonFp)

	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	// Create and register a Consumer FP next
	consumerFpSk, _, err := datagen.GenRandomBTCKeyPair(s.r)
	s.Require().NoError(err)

	consumerFp := chain.CreateConsumerFpFromNodeAddr(
		s.T(),
		s.r,
		ConsumerID,
		consumerFpSk,
		nonValidatorNode,
	)
	s.Require().NotNil(consumerFp)

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
			babylonFp,
			consumerFp,
		},
		s.delBTCSK,
		nonValidatorNode.PublicAddress,
		stakingTimeBlocks,
		s.stakingValue,
	)

	// Check babylon delegation
	pendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(babylonFp.BtcPk.MarshalHex())
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
