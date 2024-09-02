package e2e

import (
	"time"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	ct "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/stretchr/testify/suite"
)

type BTCTimestampingPhase2RlyTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *BTCTimestampingPhase2RlyTestSuite) SetupSuite() {
	s.T().Log("setting up phase 2 go relayer integration test suite...")
	var (
		err error
	)

	// The e2e test flow is as follows:
	//
	// 1. Configure two chains - chain A and chain B.
	//   * For each chain, set up several validator nodes
	//   * Initialize configs and genesis for all them.
	// 2. Start both networks.
	// 3. Store and instantiate babylon contract on chain B.
	// 3. Execute various e2e tests, excluding IBC
	s.configurer, err = configurer.NewBTCTimestampingPhase2RlyConfigurer(s.T(), true)

	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *BTCTimestampingPhase2RlyTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *BTCTimestampingPhase2RlyTestSuite) Test1IbcCheckpointingPhase2Rly() {
	chainA := s.configurer.GetChainConfig(0)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	babylonNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	czNode, err := s.configurer.GetChainConfig(1).GetNodeAtIndex(2)
	s.NoError(err)

	// Validate channel state and kind (Babylon side)
	// Wait until the channel (Babylon side) is open
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

	// Get the client ID under this IBC channel
	channelClientState, err := nonValidatorNode.QueryChannelClientState(babylonChannel.ChannelId, babylonChannel.PortId)
	s.NoError(err)
	clientID := channelClientState.IdentifiedClientState.ClientId

	// Query checkpoint chain info for the consumer chain
	listHeaderResp, err := babylonNode.QueryListHeaders(clientID, &query.PageRequest{Limit: 1})
	s.NoError(err)
	s.GreaterOrEqual(len(listHeaderResp.Headers), 1)
	startEpochNum := listHeaderResp.Headers[0].BabylonEpoch
	endEpochNum := startEpochNum + 2

	// wait until epoch endEpochNum
	// so that there will be endEpochNum - startEpochNum + 1 = 3
	// BTC timestamps in Babylon contract
	chainA.WaitUntilHeight(int64(endEpochNum*10 + 5))
	babylonNode.FinalizeSealedEpochs(1, endEpochNum)

	// ensure endEpochNum has been finalised
	endEpoch, err := babylonNode.QueryRawCheckpoint(endEpochNum)
	s.NoError(err)
	s.Equal(endEpoch.Status, ct.Finalized)

	// there should be 3 IBC packets sent (with sequence number 1, 2, 3).
	// Thus, the next sequence number will eventually be 4
	s.Eventually(func() bool {
		nextSequenceSendResp, err := babylonNode.QueryNextSequenceSend(babylonChannel.ChannelId, babylonChannel.PortId)
		if err != nil {
			return false
		}
		s.T().Logf("next sequence send at ZoneConcierge is %d", nextSequenceSendResp.NextSequenceSend)
		return nextSequenceSendResp.NextSequenceSend >= endEpochNum-startEpochNum+1+1
	}, time.Minute, time.Second*2)

	// ensure the next receive sequence number of Babylon contract is also 3
	var nextSequenceRecv *channeltypes.QueryNextSequenceReceiveResponse
	s.Eventually(func() bool {
		nextSequenceRecv, err = czNode.QueryNextSequenceReceive(babylonChannel.Counterparty.ChannelId, babylonChannel.Counterparty.PortId)
		if err != nil {
			return false
		}
		s.T().Logf("next sequence receive at Babylon contract is %d", nextSequenceRecv.NextSequenceReceive)
		return nextSequenceRecv.NextSequenceReceive >= endEpochNum-startEpochNum+1+1
	}, time.Minute, time.Second*2)

	// Ensure the IBC packet acknowledgements (on chain B) are there
	nextSequence := nextSequenceRecv.NextSequenceReceive
	for seq := uint64(1); seq < nextSequence; seq++ {
		var seqResp *channeltypes.QueryPacketAcknowledgementResponse
		s.Eventually(func() bool {
			seqResp, err = czNode.QueryPacketAcknowledgement(czChannel.ChannelId, czChannel.PortId, seq)
			s.T().Logf("acknowledgement resp of IBC packet #%d: %v, err: %v", seq, seqResp, err)
			return err == nil
		}, time.Minute, time.Second*2)
	}
}
