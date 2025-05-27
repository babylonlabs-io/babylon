package e2e

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"strconv"
	"time"

	"github.com/stretchr/testify/suite"

	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
)

const (
	ConsumerID = "optimism-1234"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))
)

type FinalityContractTestSuite struct {
	suite.Suite

	configurer configurer.Configurer

	// Cross-test config data
	FinalityContractAddr string
}

func (s *FinalityContractTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var (
		err error
	)

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
		datagen.GenRandomHexStr(r, 5),
		"Chain description: "+datagen.GenRandomHexStr(r, 15),
		3,
	)

	validatorNode, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(0)
	require.NoError(s.T(), err)

	// TODO: Register the Consumer through a gov proposal
	validatorNode.RegisterRollupConsumerChain(initialization.ValidatorWalletName, registeredConsumer.ConsumerId, registeredConsumer.ConsumerName, registeredConsumer.ConsumerDescription, s.FinalityContractAddr)

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
