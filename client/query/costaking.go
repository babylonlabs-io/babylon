package query

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"

	costakingtypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

// QueryCostaking queries the Costaking module of the Babylon node according to the given function
func (c *QueryClient) QueryCostaking(f func(ctx context.Context, queryClient costakingtypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := costakingtypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

// CostakingParams queries the costaking module parameters
func (c *QueryClient) CostakingParams() (*costakingtypes.QueryParamsResponse, error) {
	var resp *costakingtypes.QueryParamsResponse
	err := c.QueryCostaking(func(ctx context.Context, queryClient costakingtypes.QueryClient) error {
		var err error
		req := &costakingtypes.QueryParamsRequest{}
		resp, err = queryClient.Params(ctx, req)
		return err
	})

	return resp, err
}

// CostakerRewardsTracker queries the costaker rewards tracker for a given address
func (c *QueryClient) CostakerRewardsTracker(costakerAddress string) (*costakingtypes.QueryCostakerRewardsTrackerResponse, error) {
	var resp *costakingtypes.QueryCostakerRewardsTrackerResponse
	err := c.QueryCostaking(func(ctx context.Context, queryClient costakingtypes.QueryClient) error {
		var err error
		req := &costakingtypes.QueryCostakerRewardsTrackerRequest{
			CostakerAddress: costakerAddress,
		}
		resp, err = queryClient.CostakerRewardsTracker(ctx, req)
		return err
	})

	return resp, err
}

// HistoricalRewards queries the historical rewards for a given period
func (c *QueryClient) HistoricalRewards(period uint64) (*costakingtypes.QueryHistoricalRewardsResponse, error) {
	var resp *costakingtypes.QueryHistoricalRewardsResponse
	err := c.QueryCostaking(func(ctx context.Context, queryClient costakingtypes.QueryClient) error {
		var err error
		req := &costakingtypes.QueryHistoricalRewardsRequest{
			Period: period,
		}
		resp, err = queryClient.HistoricalRewards(ctx, req)
		return err
	})

	return resp, err
}

// CurrentRewards queries the current rewards for the costaking pool
func (c *QueryClient) CurrentRewards() (*costakingtypes.QueryCurrentRewardsResponse, error) {
	var resp *costakingtypes.QueryCurrentRewardsResponse
	err := c.QueryCostaking(func(ctx context.Context, queryClient costakingtypes.QueryClient) error {
		var err error
		req := &costakingtypes.QueryCurrentRewardsRequest{}
		resp, err = queryClient.CurrentRewards(ctx, req)
		return err
	})

	return resp, err
}
