package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) RewardGauges(goCtx context.Context, req *types.QueryRewardGaugesRequest) (*types.QueryRewardGaugesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	// try to cast address
	address, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	rgMap := map[string]*types.RewardGauge{}

	// find reward gauge
	for _, sType := range types.GetAllStakeholderTypes() {
		rg := k.GetRewardGauge(ctx, sType, address)
		if rg == nil {
			continue
		}
		rgMap[sType.String()] = rg
	}

	// return error if no reward gauge is found
	if len(rgMap) == 0 {
		return nil, types.ErrRewardGaugeNotFound
	}

	return &types.QueryRewardGaugesResponse{RewardGauges: convertToRewardGaugesResponse(rgMap)}, nil
}

func (k Keeper) BTCStakingGauge(goCtx context.Context, req *types.QueryBTCStakingGaugeRequest) (*types.QueryBTCStakingGaugeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	// find gauge
	gauge := k.GetBTCStakingGauge(ctx, req.Height)
	if gauge == nil {
		return nil, types.ErrBTCStakingGaugeNotFound
	}

	return &types.QueryBTCStakingGaugeResponse{Gauge: convertGaugeToBTCStakingResponse(*gauge)}, nil
}

func (k Keeper) BTCTimestampingGauge(goCtx context.Context, req *types.QueryBTCTimestampingGaugeRequest) (*types.QueryBTCTimestampingGaugeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	// find gauge
	gauge := k.GetBTCTimestampingGauge(ctx, req.EpochNum)
	if gauge == nil {
		return nil, types.ErrBTCTimestampingGaugeNotFound
	}

	return &types.QueryBTCTimestampingGaugeResponse{Gauge: convertGaugeToBTCTimestampingResponse(*gauge)}, nil
}

func convertGaugeToBTCStakingResponse(gauge types.Gauge) *types.BTCStakingGaugeResponse {
	return &types.BTCStakingGaugeResponse{
		Coins: gauge.Coins,
	}
}

func convertGaugeToBTCTimestampingResponse(gauge types.Gauge) *types.BTCTimestampingGaugeResponse {
	return &types.BTCTimestampingGaugeResponse{
		Coins: gauge.Coins,
	}
}

func convertToRewardGaugesResponse(rgMap map[string]*types.RewardGauge) map[string]*types.RewardGaugesResponse {
	rewardGuagesResponse := make(map[string]*types.RewardGaugesResponse)
	for stakeholderType, rg := range rgMap {
		rewardGuagesResponse[stakeholderType] = &types.RewardGaugesResponse{
			Coins:          rg.Coins,
			WithdrawnCoins: rg.WithdrawnCoins,
		}
	}
	return rewardGuagesResponse
}
