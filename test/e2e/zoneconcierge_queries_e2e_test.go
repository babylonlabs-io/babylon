package e2e

import (
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
)

type ZoneConciergeQueriesTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *ZoneConciergeQueriesTestSuite) SetupSuite() {
	s.T().Log("setting up zoneconcierge queries e2e test suite...")
	var err error

	s.configurer, err = configurer.NewBTCTimestampingConfigurer(s.T(), true)
	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *ZoneConciergeQueriesTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	s.Require().NoError(err)
}

func (s *ZoneConciergeQueriesTestSuite) TestZoneConciergeQueries() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	s.T().Log("Testing QueryZoneConciergeParams...")
	params := nonValidatorNode.QueryZoneConciergeParams()
	s.NotNil(params)
	s.T().Logf("ZoneConcierge params: %+v", params)

	s.T().Log("Testing QueryFinalizedBSNsInfo with empty consumer IDs...")
	finalizedResp := nonValidatorNode.QueryFinalizedBSNsInfo([]string{}, false)
	s.NotNil(finalizedResp)
	s.T().Logf("Finalized BSNs info: %+v", finalizedResp)

	testConsumerID := "test-consumer-1"
	s.T().Log("Testing QueryFinalizedBSNsInfo with test consumer ID...")
	finalizedRespWithID := nonValidatorNode.QueryFinalizedBSNsInfo([]string{testConsumerID}, true)
	s.NotNil(finalizedRespWithID)
	s.T().Logf("Finalized BSNs info for %s: %+v", testConsumerID, finalizedRespWithID)

	s.T().Log("Testing QueryLatestEpochHeader...")
	headerResp := nonValidatorNode.QueryLatestEpochHeader(testConsumerID)
	s.NotNil(headerResp)
	s.T().Logf("Latest epoch header for %s: %+v", testConsumerID, headerResp)

	s.T().Log("Testing QueryBSNLastSentSegment...")
	segmentResp := nonValidatorNode.QueryBSNLastSentSegment(testConsumerID)
	s.NotNil(segmentResp)
	s.T().Logf("BSN last sent segment for %s: %+v", testConsumerID, segmentResp)

	s.T().Log("Testing QueryGetSealedEpochProof...")
	epochNum := uint64(1)
	proofResp := nonValidatorNode.QueryGetSealedEpochProof(epochNum)
	s.NotNil(proofResp)
	s.T().Logf("Sealed epoch proof for epoch %d: %+v", epochNum, proofResp)

	chainA.WaitUntilHeight(10)

	s.T().Log("Testing QueryGetSealedEpochProof for epoch 0...")
	proofResp0 := nonValidatorNode.QueryGetSealedEpochProof(0)
	s.NotNil(proofResp0)
	s.T().Logf("Sealed epoch proof for epoch 0: %+v", proofResp0)

	s.T().Log("All zoneconcierge query tests completed successfully!")
}

func (s *ZoneConciergeQueriesTestSuite) TestZoneConciergeQueriesWithMultipleConsumers() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(5)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	consumerIDs := []string{"consumer-1", "consumer-2", "consumer-3"}

	s.T().Log("Testing QueryFinalizedBSNsInfo with multiple consumer IDs...")
	finalizedResp := nonValidatorNode.QueryFinalizedBSNsInfo(consumerIDs, false)
	s.NotNil(finalizedResp)
	s.T().Logf("Finalized BSNs info for multiple consumers: %+v", finalizedResp)

	for _, consumerID := range consumerIDs {
		s.T().Logf("Testing queries for consumer ID: %s", consumerID)

		headerResp := nonValidatorNode.QueryLatestEpochHeader(consumerID)
		s.NotNil(headerResp)

		segmentResp := nonValidatorNode.QueryBSNLastSentSegment(consumerID)
		s.NotNil(segmentResp)

		time.Sleep(100 * time.Millisecond)
	}

	s.T().Log("Multiple consumer query tests completed successfully!")
}
