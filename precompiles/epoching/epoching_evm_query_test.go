package epoching_test

import (
	epochingpc "github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func (s *PrecompileIntegrationTestSuite) SetupTest() {
	s.BaseTestSuite.SetupApp(s.T())

	a, err := epochingpc.LoadABI()
	require.NoError(s.T(), err)
	s.abi = a
	s.addr = common.HexToAddress(epochingpc.EpochingPrecompileAddress)
}

func (s *PrecompileIntegrationTestSuite) TestCurrentEpoch_QueryContract() {
	resp, err := s.QueryContract(s.addr, s.abi, epochingpc.CurrentEpochMethod)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	var out epochingpc.CurrentEpochOutput
	err = s.abi.UnpackIntoInterface(&out, epochingpc.CurrentEpochMethod, resp.Ret)
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(out.Response.CurrentEpoch, uint64(1))
	s.Require().GreaterOrEqual(out.Response.EpochBoundary, uint64(10))
}
