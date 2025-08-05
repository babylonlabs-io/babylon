package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
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
		if err := k.sendAllBtcDelegationTypeToRewardsGauge(ctx, sType, address); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

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

// DelegationRewards returns the current rewards for the specified finality provider and delegator
func (k Keeper) DelegationRewards(ctx context.Context, req *types.QueryDelegationRewardsRequest) (*types.QueryDelegationRewardsResponse, error) {
	// try to cast address
	fpAddr, err := sdk.AccAddressFromBech32(req.FinalityProviderAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	delAddr, err := sdk.AccAddressFromBech32(req.DelegatorAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Finalize the period to get a new history with the current rewards available
	// This will not be committed anyways because it is a query
	endPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fpAddr)
	if err != nil {
		return nil, err
	}

	rewards, err := k.CalculateBTCDelegationRewards(ctx, fpAddr, delAddr, endPeriod)
	if err != nil {
		return nil, err
	}

	return &types.QueryDelegationRewardsResponse{Rewards: rewards}, nil
}

// FpCurrentRewards gets the current finality provider rewards.
func (k Keeper) FpCurrentRewards(ctx context.Context, req *types.QueryFpCurrentRewardsRequest) (*types.QueryFpCurrentRewardsResponse, error) {
	fpAddr, err := sdk.AccAddressFromBech32(req.FinalityProviderAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	fpCurrRwds, err := k.GetFinalityProviderCurrentRewards(ctx, fpAddr)
	if err != nil {
		return nil, types.ErrFPCurrentRewardsInvalid.Wrapf("failed to get for addr %s: %s", fpAddr.String(), err.Error())
	}

	return fpCurrRwds.ToResponse(), nil
}

func convertGaugeToBTCStakingResponse(gauge types.Gauge) *types.BTCStakingGaugeResponse {
	return &types.BTCStakingGaugeResponse{
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
