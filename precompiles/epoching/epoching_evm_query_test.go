package epoching_test

import (
	"github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
)

func (s *PrecompileIntegrationTestSuite) TestCurrentEpoch_QueryContract() {
	resp, err := s.QueryContract(s.addr, s.abi, epoching.CurrentEpochMethod)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	var out epoching.CurrentEpochOutput
	err = s.abi.UnpackIntoInterface(&out, epoching.CurrentEpochMethod, resp.Ret)
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(out.Response.CurrentEpoch, uint64(1))
	s.Require().GreaterOrEqual(out.Response.EpochBoundary, uint64(10))
}
