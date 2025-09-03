package epoching_test

import (
	"github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
)

func (ts *PrecompileIntegrationTestSuite) TestCurrentEpoch_QueryContract() {
	resp, err := ts.QueryContract(ts.addr, ts.abi, epoching.CurrentEpochMethod)
	ts.Require().NoError(err)
	ts.Require().NotNil(resp)

	var out epoching.CurrentEpochOutput
	err = ts.abi.UnpackIntoInterface(&out, epoching.CurrentEpochMethod, resp.Ret)
	ts.Require().NoError(err)
	ts.Require().GreaterOrEqual(out.Response.CurrentEpoch, uint64(1))
	ts.Require().GreaterOrEqual(out.Response.EpochBoundary, uint64(10))
}
