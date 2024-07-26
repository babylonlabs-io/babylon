package keeper_test

import (
	"testing"

	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/stretchr/testify/require"
)

func TestParamsQuery(t *testing.T) {
	keeper, ctx := testkeeper.IncentiveKeeper(t, nil, nil, nil)
	params := types.DefaultParams()
	err := keeper.SetParams(ctx, params)
	require.NoError(t, err)

	response, err := keeper.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}
