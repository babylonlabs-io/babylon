package e2e

import (
	"strconv"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
)

type FinalityContractTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
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
			"consumer_id": "optimism-1234",
			"is_enabled": true
		}`,
		initialization.ValidatorWalletName,
	)

	var contracts []string
	s.Eventually(func() bool {
		contracts, err = nonValidatorNode.QueryContractsFromId(latestWasmId)
		return err == nil && len(contracts) == 1
	}, time.Second*10, time.Second)
	contractAddr := contracts[0]
	s.T().Log("Finality gadget contract address: ", contractAddr)
}
