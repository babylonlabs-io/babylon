package e2e

import (
	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/stretchr/testify/suite"
)

type VoteExtensionTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *VoteExtensionTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var (
		err error
	)

	s.configurer, err = configurer.NewVoteExtensionConfigurer(s.T(), true)
	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *VoteExtensionTestSuite) TearDownSuite() {
	// err := s.configurer.ClearResources()
	// if err != nil {
	// 	s.T().Logf("error to clear resources %s", err.Error())
	// }
}

func (s *VoteExtensionTestSuite) Test1VoteExtension() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(5)
	s.T().Log("Block height 5 reached")
}
