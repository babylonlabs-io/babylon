package keeper

import (
	"context"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

const maxQueryConsumersRegistryLimit = 100

func (k Keeper) ConsumerRegistryList(c context.Context, req *types.QueryConsumerRegistryListRequest) (*types.QueryConsumerRegistryListResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	var consumerRegisters []*types.ConsumerRegisterResponse
	var count uint64
	limit := uint64(100) // default limit
	if req.Pagination != nil && req.Pagination.Limit > 0 {
		limit = req.Pagination.Limit
	}

	// Collect consumers up to the limit (simple approach for collections migration)
	err := k.ConsumerRegistry.Walk(ctx, nil, func(consumerID string, consumerRegister types.ConsumerRegister) (bool, error) {
		if count >= limit {
			return true, nil // stop iteration
		}
		consumerRegisters = append(consumerRegisters, consumerRegister.ToResponse())
		count++
		return false, nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pageRes := &query.PageResponse{
		Total: count,
	}

	resp := &types.QueryConsumerRegistryListResponse{
		ConsumerRegisters: consumerRegisters,
		Pagination:        pageRes,
	}
	return resp, nil
}

// ConsumersRegistry returns the registration for a given list of consumers
func (k Keeper) ConsumersRegistry(c context.Context, req *types.QueryConsumersRegistryRequest) (*types.QueryConsumersRegistryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	// return if no consumer IDs are provided
	if len(req.ConsumerIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "consumer IDs cannot be empty")
	}

	// return if consumer IDs exceed the limit
	if len(req.ConsumerIds) > maxQueryConsumersRegistryLimit {
		return nil, status.Errorf(codes.InvalidArgument, "cannot query more than %d consumers", maxQueryConsumersRegistryLimit)
	}

	// return if consumer IDs contain duplicates or empty strings
	if err := bbn.CheckForDuplicatesAndEmptyStrings(req.ConsumerIds); err != nil {
		return nil, status.Error(codes.InvalidArgument, types.ErrInvalidConsumerIDs.Wrap(err.Error()).Error())
	}

	ctx := sdk.UnwrapSDKContext(c)
	consumersRegisters := []*types.ConsumerRegisterResponse{}
	for _, consumerID := range req.ConsumerIds {
		consumerRegister, err := k.GetConsumerRegister(ctx, consumerID)
		if err != nil {
			return nil, err
		}

		consumersRegisters = append(consumersRegisters, consumerRegister.ToResponse())
	}

	resp := &types.QueryConsumersRegistryResponse{ConsumerRegisters: consumersRegisters}
	return resp, nil
}
