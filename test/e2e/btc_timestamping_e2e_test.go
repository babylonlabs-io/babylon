package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	bbn "github.com/babylonlabs-io/babylon/types"
	ct "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	itypes "github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/stretchr/testify/suite"
)

type BTCTimestampingTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *BTCTimestampingTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var (
		err error
	)

	// The e2e test flow is as follows:
	//
	// 1. Configure two chains - chan A and chain B.
	//   * For each chain, set up several validator nodes
	//   * Initialize configs and genesis for all them.
	// 2. Start both networks.
	// 3. Run IBC relayer between the two chains.
	// 4. Execute various e2e tests, including IBC
	s.configurer, err = configurer.NewBTCTimestampingConfigurer(s.T(), true)
	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *BTCTimestampingTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// Most simple test, just checking that two chains are up and connected through
// ibc
func (s *BTCTimestampingTestSuite) Test1ConnectIbc() {
	chainA := s.configurer.GetChainConfig(0)
	chainB := s.configurer.GetChainConfig(1)
	_, err := chainA.GetDefaultNode()
	s.NoError(err)
	_, err = chainB.GetDefaultNode()
	s.NoError(err)
}

func (s *BTCTimestampingTestSuite) Test2BTCBaseHeader() {
	hardcodedHeader, _ := bbn.NewBTCHeaderBytesFromHex("0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a45068653ffff7f2002000000")
	hardcodedHeaderHeight := uint32(0)

	chainA := s.configurer.GetChainConfig(0)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	baseHeader, err := nonValidatorNode.QueryBtcBaseHeader()
	s.NoError(err)
	s.Equal(baseHeader.HeaderHex, hardcodedHeader.MarshalHex())
	s.Equal(hardcodedHeaderHeight, baseHeader.Height)
}

func (s *BTCTimestampingTestSuite) Test3SendTx() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	chainA := s.configurer.GetChainConfig(0)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	tip1, err := nonValidatorNode.QueryTip()
	s.NoError(err)

	nonValidatorNode.InsertNewEmptyBtcHeader(r)

	tip2, err := nonValidatorNode.QueryTip()
	s.NoError(err)

	s.Equal(tip1.Height+1, tip2.Height)

	// check that light client properly updates its state
	tip1Depth, err := nonValidatorNode.QueryHeaderDepth(tip1.HashHex)
	s.NoError(err)
	s.Equal(tip1Depth, uint32(1))

	tip2Depth, err := nonValidatorNode.QueryHeaderDepth(tip2.HashHex)
	s.NoError(err)
	// tip should have 0 depth
	s.Equal(tip2Depth, uint32(0))
}

func (s *BTCTimestampingTestSuite) Test4FinalizeEpochs() {
	chainA := s.configurer.GetChainConfig(0)

	chainA.WaitUntilHeight(35)

	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// Finalize epoch 1, 2, 3
	var (
		startEpochNum uint64 = 1
		endEpochNum   uint64 = 3
	)

	nonValidatorNode.FinalizeSealedEpochs(startEpochNum, endEpochNum)

	endEpoch, err := nonValidatorNode.QueryRawCheckpoint(endEpochNum)
	s.NoError(err)
	s.Equal(endEpoch.Status, ct.Finalized)

	// Wait for a some time to ensure that the checkpoint is included in the chain
	time.Sleep(20 * time.Second)
	// Wait for next block
	nonValidatorNode.WaitForNextBlock()
}

func (s *BTCTimestampingTestSuite) Test5Wasm() {
	contractPath := "/bytecode/storage_contract.wasm"
	chainA := s.configurer.GetChainConfig(0)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// store the wasm code
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

	// instantiate the wasm contract
	var contracts []string
	nonValidatorNode.InstantiateWasmContract(
		strconv.Itoa(latestWasmId),
		`{}`,
		initialization.ValidatorWalletName,
	)
	s.Eventually(func() bool {
		contracts, err = nonValidatorNode.QueryContractsFromId(latestWasmId)
		return err == nil && len(contracts) == 1
	}, time.Second*10, time.Second)
	contractAddr := contracts[0]

	// execute contract
	data := []byte{1, 2, 3, 4, 5}
	dataHex := hex.EncodeToString(data)
	dataHash := sha256.Sum256(data)
	dataHashHex := hex.EncodeToString(dataHash[:])
	storeMsg := fmt.Sprintf(`{"save_data": { "data": "%s" } }`, dataHex)
	nonValidatorNode.WasmExecute(contractAddr, storeMsg, initialization.ValidatorWalletName)

	// the data is eventually included in the contract
	queryMsg := fmt.Sprintf(`{"check_data": { "data_hash": "%s" } }`, dataHashHex)
	var queryResult map[string]interface{}
	s.Eventually(func() bool {
		queryResult, err = nonValidatorNode.QueryWasmSmartObject(contractAddr, queryMsg)
		return err == nil
	}, time.Second*10, time.Second)

	finalized := queryResult["finalized"].(bool)
	latestFinalizedEpoch := int(queryResult["latest_finalized_epoch"].(float64))
	saveEpoch := int(queryResult["save_epoch"].(float64))

	s.False(finalized)
	// in previous test we already finalized epoch 3
	s.Equal(3, latestFinalizedEpoch)
	// data is not finalized yet, so save epoch should be strictly greater than latest finalized epoch
	s.Greater(saveEpoch, latestFinalizedEpoch)
}

func (s *BTCTimestampingTestSuite) Test6InterceptFeeCollector() {
	chainA := s.configurer.GetChainConfig(0)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// ensure incentive module account has positive balance
	incentiveModuleAddr, err := nonValidatorNode.QueryModuleAddress(itypes.ModuleName)
	s.NoError(err)
	incentiveBalance, err := nonValidatorNode.QueryBalances(incentiveModuleAddr.String())
	s.NoError(err)
	s.NotEmpty(incentiveBalance)
	s.T().Logf("incentive module account's balance: %s", incentiveBalance.String())
	s.True(incentiveBalance.IsAllPositive())

	// ensure BTC staking gauge at the current height is eventually non-empty
	// NOTE: sometimes incentive module's BeginBlock is not triggered yet. If this
	// happens, we might need to wait for some time.
	curHeight, err := nonValidatorNode.QueryCurrentHeight()
	s.NoError(err)
	s.Eventually(func() bool {
		btcStakingGauge, err := nonValidatorNode.QueryBTCStakingGauge(uint64(curHeight))
		if err != nil {
			return false
		}
		s.T().Logf("BTC staking gauge at current height %d: %s", curHeight, btcStakingGauge.String())
		return len(btcStakingGauge.Coins) >= 1 && btcStakingGauge.Coins[0].Amount.IsPositive()
	}, time.Second*10, time.Second)

	// after 1 block, incentive's balance has to be accumulated
	nonValidatorNode.WaitForNextBlock()
	incentiveBalance2, err := nonValidatorNode.QueryBalances(incentiveModuleAddr.String())
	s.NoError(err)
	s.T().Logf("incentive module account's balance after a block: %s", incentiveBalance2.String())
	s.True(incentiveBalance2.IsAllGTE(incentiveBalance))
}
