package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	btcstaking "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

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

	consumerRegisters := []*types.ConsumerRegisterResponse{}
	store := k.consumerRegistryStore(ctx)
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		consumerID := string(key)
		consumerRegister, err := k.GetConsumerRegister(ctx, consumerID)
		if err != nil {
			return err
		}
		consumerRegisters = append(consumerRegisters, consumerRegister.ToResponse())
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
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

// FinalityProviders returns a paginated list of all registered finality providers for a given consumer
func (k Keeper) FinalityProviders(c context.Context, req *types.QueryFinalityProvidersRequest) (*types.QueryFinalityProvidersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	store := k.finalityProviderStore(ctx, req.ConsumerId)

	var fpResp []*types.FinalityProviderResponse
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		var fp btcstaking.FinalityProvider
		if err := fp.Unmarshal(value); err != nil {
			return err
		}

		resp := types.NewFinalityProviderResponse(&fp)
		fpResp = append(fpResp, resp)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryFinalityProvidersResponse{FinalityProviders: fpResp, Pagination: pageRes}, nil
}

// FinalityProvider returns the finality provider with the specified finality provider BTC PK
func (k Keeper) FinalityProvider(c context.Context, req *types.QueryFinalityProviderRequest) (*types.QueryFinalityProviderResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if len(req.FpBtcPkHex) == 0 {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "finality provider BTC public key cannot be empty")
	}

	fpPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c)
	fp, err := k.GetConsumerFinalityProvider(ctx, req.ConsumerId, fpPK)
	if err != nil {
		return nil, err
	}

	fpResp := types.NewFinalityProviderResponse(fp)
	return &types.QueryFinalityProviderResponse{FinalityProvider: fpResp}, nil
}

// FinalityProviderConsumer returns the consumer ID for the finality provider with the specified finality provider BTC PK
func (k Keeper) FinalityProviderConsumer(c context.Context, req *types.QueryFinalityProviderConsumerRequest) (*types.QueryFinalityProviderConsumerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if len(req.FpBtcPkHex) == 0 {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "finality provider BTC public key cannot be empty")
	}

	fpPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c)
	consumerID, err := k.GetConsumerOfFinalityProvider(ctx, fpPK)
	if err != nil {
		return nil, err
	}

	return &types.QueryFinalityProviderConsumerResponse{ConsumerId: consumerID}, nil
}
