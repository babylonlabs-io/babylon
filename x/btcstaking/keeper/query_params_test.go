package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func TestParamsQuery(t *testing.T) {
	keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)
	params := types.DefaultParams()

	currParams := keeper.GetParams(ctx)
	params.BtcActivationHeight = currParams.BtcActivationHeight + 1
	err := keeper.SetParams(ctx, params)
	require.NoError(t, err)

	response, err := keeper.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}

func TestParamsByVersionQuery(t *testing.T) {
	keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)

	currParams := keeper.GetParams(ctx)

	// starting with `1` as BTCStakingKeeper creates params with version 0
	params1 := types.DefaultParams()
	params1.UnbondingTimeBlocks = 10000
	params1.BtcActivationHeight = currParams.BtcActivationHeight + 1

	params2 := types.DefaultParams()
	params2.UnbondingTimeBlocks = 20000
	params2.BtcActivationHeight = currParams.BtcActivationHeight + 2

	params3 := types.DefaultParams()
	params3.UnbondingTimeBlocks = 30000
	params3.BtcActivationHeight = currParams.BtcActivationHeight + 3
	// Check that after update we always return the latest version of params through Params query
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

func TestParamsByBTCHeightQuery(t *testing.T) {
	keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)
	currParams := keeper.GetParams(ctx)

	// starting with `1` as BTCStakingKeeper creates params with version 0
	params1 := types.DefaultParams()
	params1.UnbondingTimeBlocks = 10000
	params1.BtcActivationHeight = currParams.BtcActivationHeight + 1

	params2 := types.DefaultParams()
	params2.UnbondingTimeBlocks = 20000
	params2.BtcActivationHeight = currParams.BtcActivationHeight + 2

	// Check that after update we always return the latest version of params through Params query
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

	resp0, err := keeper.ParamsByBTCHeight(ctx, &types.QueryParamsByBTCHeightRequest{BtcHeight: params1.BtcActivationHeight})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsByBTCHeightResponse{Params: params1, Version: 1}, resp0)

	resp1, err := keeper.ParamsByBTCHeight(ctx, &types.QueryParamsByBTCHeightRequest{BtcHeight: params2.BtcActivationHeight})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsByBTCHeightResponse{Params: params2, Version: 2}, resp1)
}
