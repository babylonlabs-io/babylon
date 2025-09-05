package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

var _ types.QueryServer = Keeper{}

// Params returns the parameters of the module
func (k Keeper) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(sdkCtx)

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// CostakerRewardsTracker returns the costaker rewards tracker for a given address
func (k Keeper) CostakerRewardsTracker(ctx context.Context, req *types.QueryCostakerRewardsTrackerRequest) (*types.QueryCostakerRewardsTrackerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.CostakerAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "costaker address cannot be empty")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	
	costakerAddr, err := sdk.AccAddressFromBech32(req.CostakerAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid costaker address")
	}

	tracker, err := k.GetCostakerRewards(sdkCtx, costakerAddr)
	if err != nil {
		return nil, status.Error(codes.NotFound, "costaker rewards tracker not found")
	}

	return &types.QueryCostakerRewardsTrackerResponse{
		StartPeriodCumulativeReward: tracker.StartPeriodCumulativeReward,
		ActiveSatoshis:             tracker.ActiveSatoshis,
		ActiveBaby:                 tracker.ActiveBaby,
		TotalScore:                 tracker.TotalScore,
	}, nil
}

// HistoricalRewards returns the historical rewards for a given period
func (k Keeper) HistoricalRewards(ctx context.Context, req *types.QueryHistoricalRewardsRequest) (*types.QueryHistoricalRewardsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	
	rewards, err := k.GetHistoricalRewards(sdkCtx, req.Period)
	if err != nil {
		return nil, status.Error(codes.NotFound, "historical rewards not found for the given period")
	}

	return &types.QueryHistoricalRewardsResponse{
		CumulativeRewardsPerScore: rewards.CumulativeRewardsPerScore,
	}, nil
}

// CurrentRewards returns the current rewards for the costaking pool
func (k Keeper) CurrentRewards(ctx context.Context, req *types.QueryCurrentRewardsRequest) (*types.QueryCurrentRewardsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	
	currentRewards, err := k.GetCurrentRewards(sdkCtx)
	if err != nil {
		return nil, status.Error(codes.NotFound, "current rewards not found")
	}

	return &types.QueryCurrentRewardsResponse{
		Rewards:    currentRewards.Rewards,
		Period:     currentRewards.Period,
		TotalScore: currentRewards.TotalScore,
	}, nil
}