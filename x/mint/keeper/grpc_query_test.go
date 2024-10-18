package keeper_test //nolint:all

import (
	gocontext "context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/testutil/helper"
	"github.com/babylonlabs-io/babylon/x/mint/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type MintTestSuite struct {
	suite.Suite

	app         *app.BabylonApp
	ctx         sdk.Context
	queryClient types.QueryClient
}

func (suite *MintTestSuite) SetupTest() {
	h := helper.NewHelper(suite.T())
	testApp, ctx := h.App, h.Ctx

	queryHelper := baseapp.NewQueryServerTestHelper(ctx, testApp.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, testApp.MintKeeper)
	queryClient := types.NewQueryClient(queryHelper)

	suite.app = testApp
	suite.ctx = ctx

	suite.queryClient = queryClient
}

func (suite *MintTestSuite) TestGRPC() {
	app, ctx, queryClient := suite.app, suite.ctx, suite.queryClient

	inflation, err := queryClient.InflationRate(gocontext.Background(), &types.QueryInflationRateRequest{})
	suite.Require().NoError(err)
	suite.Require().Equal(inflation.InflationRate, app.MintKeeper.GetMinter(ctx).InflationRate)

	annualProvisions, err := queryClient.AnnualProvisions(gocontext.Background(), &types.QueryAnnualProvisionsRequest{})
	suite.Require().NoError(err)
	suite.Require().Equal(annualProvisions.AnnualProvisions, app.MintKeeper.GetMinter(ctx).AnnualProvisions)

	genesisTime, err := queryClient.GenesisTime(gocontext.Background(), &types.QueryGenesisTimeRequest{})
	suite.Require().NoError(err)
	suite.Require().Equal(genesisTime.GenesisTime, app.MintKeeper.GetGenesisTime(ctx).GenesisTime)
}

func TestMintTestSuite(t *testing.T) {
	suite.Run(t, new(MintTestSuite))
}
