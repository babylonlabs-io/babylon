package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func TestParamsQuery(t *testing.T) {
	keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)
	params := types.DefaultParams()

	err := keeper.SetParams(ctx, params)
	require.NoError(t, err)

	response, err := keeper.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}

func TestParamsByVersionQuery(t *testing.T) {
	keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)

	// starting with `1` as BTCStakingKeeper creates params with version 0
	params1 := types.DefaultParams()
	params1.MinUnbondingTimeBlocks = 10000
	params2 := types.DefaultParams()
	params2.MinUnbondingTimeBlocks = 20000
	params3 := types.DefaultParams()
	params3.MinUnbondingTimeBlocks = 30000

	// Check that after update we always return the latest version of params throuh Params query
	err := keeper.SetParams(ctx, params1)
	require.NoError(t, err)
	response, err := keeper.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params1}, response)

	err = keeper.SetParams(ctx, params2)
	require.NoError(t, err)
	response, err = keeper.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params2}, response)

	err = keeper.SetParams(ctx, params3)
	require.NoError(t, err)
	response, err = keeper.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params3}, response)

	// Check that each past version is available through ParamsByVersion query
	resp0, err := keeper.ParamsByVersion(ctx, &types.QueryParamsByVersionRequest{Version: 1})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsByVersionResponse{Params: params1}, resp0)

	resp1, err := keeper.ParamsByVersion(ctx, &types.QueryParamsByVersionRequest{Version: 2})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsByVersionResponse{Params: params2}, resp1)

	resp2, err := keeper.ParamsByVersion(ctx, &types.QueryParamsByVersionRequest{Version: 3})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsByVersionResponse{Params: params3}, resp2)
}
