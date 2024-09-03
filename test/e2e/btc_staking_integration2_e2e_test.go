package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	wasmparams "github.com/CosmWasm/wasmd/app/params"
	bcdapp "github.com/babylonlabs-io/babylon-sdk/demo/app"
	bcdparams "github.com/babylonlabs-io/babylon-sdk/demo/app/params"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/babylon"
	cwconfig "github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/config"
	"github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/cosmwasm"
	cwcc "github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/cosmwasm"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type BTCStakingIntegration2TestSuite struct {
	suite.Suite

	babylonRPC1      string
	babylonRPC2      string
	consumerChainRPC string

	babylonController  *babylon.BabylonController
	cosmwasmController *cosmwasm.CosmwasmConsumerController
}

func (s *BTCStakingIntegration2TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")

	// Run the start-integration-test make target
	//cmd := exec.Command("make", "-C", "consumer", "start-integration-test")
	//output, err := cmd.CombinedOutput()
	//s.Require().NoError(err, "Failed to run start-integration-test: %s", output)

	s.T().Log("Integration test environment started")

	// Set the RPC URLs for the Babylon nodes and consumer chain
	s.babylonRPC1 = "http://localhost:26657"
	s.babylonRPC2 = "http://localhost:26667"
	s.consumerChainRPC = "http://localhost:26677"

	// Check if the RPC endpoints are accessible and running
	s.Require().Eventually(func() bool {
		status1, ok1 := s.checkNodeStatus(s.babylonRPC1)
		status2, ok2 := s.checkNodeStatus(s.babylonRPC2)
		status3, ok3 := s.checkNodeStatus(s.consumerChainRPC)

		s.T().Logf("Babylon Node 1 Status: %s", status1)
		s.T().Logf("Babylon Node 2 Status: %s", status2)
		s.T().Logf("Consumer Chain Status: %s", status3)

		return ok1 && ok2 && ok3
	}, 2*time.Minute, 5*time.Second, "Chain RPC endpoints not accessible or not running")

	err := s.initBabylonController()
	s.Require().NoError(err, "Failed to initialize BabylonController")

	err = s.initCosmwasmController()
	s.Require().NoError(err, "Failed to initialize CosmwasmConsumerController")
}

func (s *BTCStakingIntegration2TestSuite) TearDownSuite() {
	s.T().Log("tearing down e2e integration test suite...")

	// Run the stop-integration-test make target
	// cmd := exec.Command("make", "-C", "../consumer", "stop-integration-test")
	// output, err := cmd.CombinedOutput()
	// if err != nil {
	// 	s.T().Logf("Failed to run stop-integration-test: %s", output)
	// }
}

func (s *BTCStakingIntegration2TestSuite) checkNodeStatus(rpcURL string) (string, bool) {
	url := fmt.Sprintf("%s/status", rpcURL)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("Error accessing %s: %v", url, err), false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Unexpected status code from %s: %d", url, resp.StatusCode), false
	}

	var result struct {
		Result struct {
			NodeInfo struct {
				Network string `json:"network"`
			} `json:"node_info"`
			SyncInfo struct {
				LatestBlockHeight string `json:"latest_block_height"`
				CatchingUp        bool   `json:"catching_up"`
			} `json:"sync_info"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Sprintf("Error decoding response from %s: %v", url, err), false
	}

	status := fmt.Sprintf("Network: %s, Latest Block Height: %s, Catching Up: %v",
		result.Result.NodeInfo.Network,
		result.Result.SyncInfo.LatestBlockHeight,
		result.Result.SyncInfo.CatchingUp)

	return status, !result.Result.SyncInfo.CatchingUp
}

// TestDummy is a simple test to ensure the test suite is running
func (s *BTCStakingIntegration2TestSuite) TestDummy() {
	s.T().Log("Running dummy test")
	s.Require().True(true, "This test should always pass")

	status, err := s.babylonController.QueryNodeStatus()
	s.Require().NoError(err, "Failed to query node status")
	s.T().Logf("Node status: %v", status.SyncInfo.LatestBlockHeight)
}

// TestSuiteSetup verifies that the SetupSuite method was called and RPC endpoints are set
func (s *BTCStakingIntegration2TestSuite) TestSuiteSetup() {
	s.T().Log("Verifying suite setup")
	s.Require().NotEmpty(s.babylonRPC1, "babylonRPC1 should be set")
	s.Require().NotEmpty(s.babylonRPC2, "babylonRPC2 should be set")
	s.Require().NotEmpty(s.consumerChainRPC, "consumerChainRPC should be set")
}

func (s *BTCStakingIntegration2TestSuite) initBabylonController() error {
	cfg := config.DefaultBabylonConfig()

	btcParams := &chaincfg.RegressionNetParams // or whichever network you're using

	logger, _ := zap.NewDevelopment()

	controller, err := babylon.NewBabylonController(&cfg, btcParams, logger)
	if err != nil {
		return err
	}

	s.babylonController = controller
	return nil
}

func (s *BTCStakingIntegration2TestSuite) initCosmwasmController() error {
	cfg := cwconfig.DefaultCosmwasmConfig()

	// Override the RPC address with the one from your test suite
	//cfg.RPCAddr = s.consumerChainRPC

	// You might need to adjust other config values as needed for your test environment

	// Create a logger
	logger, _ := zap.NewDevelopment()

	// // You'll need to provide the correct encoding config
	// // This is typically available from your app's setup
	// encodingConfig := wasmparams.MakeEncodingConfig()

	sdk.SetAddrCacheEnabled(false)
	bcdparams.SetAddressPrefixes()
	tempApp := bcdapp.NewTmpApp()
	//tempApp := wasmapp.NewWasmApp(sdklogs.NewNopLogger(), dbm.NewMemDB(), nil, false, simtestutil.NewAppOptionsWithFlagHome(s.T().TempDir()), []wasmkeeper.Option{})
	encodingCfg := wasmparams.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	wcc, err := cwcc.NewCosmwasmConsumerController(cfg, encodingCfg, logger)
	require.NoError(s.T(), err)

	s.cosmwasmController = wcc
	return nil
}

func (s *BTCStakingIntegration2TestSuite) TestConsumerChainInteraction() {
	// Use Babylon controller
	babylonStatus, err := s.babylonController.QueryNodeStatus()
	s.Require().NoError(err, "Failed to query Babylon node status")
	s.T().Logf("Babylon node status: %v", babylonStatus.SyncInfo.LatestBlockHeight)

	// Use Cosmwasm controller
	consumerStatus, err := s.cosmwasmController.GetCometNodeStatus()
	s.Require().NoError(err, "Failed to query Consumer node status")
	s.T().Logf("Consumer node status: %v", consumerStatus.SyncInfo.LatestBlockHeight)
	// Add your test assertions here
	// ...
}

func (s *BTCStakingIntegration2TestSuite) getIBCClientID() string {
	// Wait for both chains to have at least one block
	s.Eventually(func() bool {
		babylonStatus, err := s.babylonController.QueryNodeStatus()
		if err != nil || babylonStatus.SyncInfo.LatestBlockHeight < 1 {
			return false
		}
		consumerStatus, err := s.cosmwasmController.GetCometNodeStatus()
		if err != nil || consumerStatus.SyncInfo.LatestBlockHeight < 1 {
			return false
		}
		return true
	}, time.Minute, time.Second*2, "Chains did not produce blocks within the expected time")

	var babylonChannel *channeltypes.IdentifiedChannel
	s.Eventually(func() bool {
		babylonChannelsResp, err := s.babylonController.IBCChannels()
		if err != nil {
			s.T().Logf("Error querying Babylon IBC channels: %v", err)
			return false
		}
		if len(babylonChannelsResp.Channels) != 1 {
			return false
		}
		babylonChannel = babylonChannelsResp.Channels[0]
		if babylonChannel.State != channeltypes.OPEN {
			return false
		}
		s.Equal(channeltypes.ORDERED, babylonChannel.Ordering)
		s.Contains(babylonChannel.Counterparty.PortId, "wasm.")
		return true
	}, time.Minute, time.Second*2, "Failed to get expected Babylon IBC channel")

	var consumerChannel *channeltypes.IdentifiedChannel
	s.Eventually(func() bool {
		consumerChannelsResp, err := s.cosmwasmController.IBCChannels()
		if err != nil {
			s.T().Logf("Error querying Consumer IBC channels: %v", err)
			return false
		}
		if len(consumerChannelsResp.Channels) != 1 {
			return false
		}
		consumerChannel = consumerChannelsResp.Channels[0]
		if consumerChannel.State != channeltypes.OPEN {
			return false
		}
		s.Equal(channeltypes.ORDERED, consumerChannel.Ordering)
		s.Equal(babylonChannel.PortId, consumerChannel.Counterparty.PortId)
		return true
	}, time.Minute, time.Second*2, "Failed to get expected Consumer IBC channel")

	// Query the channel client state
	consumerChannelState, err := s.cosmwasmController.QueryChannelClientState(consumerChannel.ChannelId, consumerChannel.PortId)
	s.Require().NoError(err, "Failed to query Consumer channel client state")

	// Query the next sequence receive
	nextSequenceRecv, err := s.cosmwasmController.QueryNextSequenceReceive(babylonChannel.Counterparty.ChannelId, babylonChannel.Counterparty.PortId)
	s.Require().NoError(err, "Failed to query next sequence receive")

	// Check that the next sequence receive is 1 (no packets sent yet)
	s.Equal(uint64(1), nextSequenceRecv.NextSequenceReceive, "Unexpected next sequence receive value")

	return consumerChannelState.IdentifiedClientState.ClientId
}
