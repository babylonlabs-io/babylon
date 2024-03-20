package keeper

import (
	"context"
	errorsmod "cosmossdk.io/errors"
	btcstaking "github.com/babylonchain/babylon/x/btcstaking/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	bbn "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

const maxQueryChainsRegistryLimit = 100

func (k Keeper) ChainRegistryList(c context.Context, req *types.QueryChainRegistryListRequest) (*types.QueryChainRegistryListResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	chainIDs := []string{}
	store := k.chainRegistryStore(ctx)
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		chainID := string(key)
		chainIDs = append(chainIDs, chainID)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &types.QueryChainRegistryListResponse{
		ChainIds:   chainIDs,
		Pagination: pageRes,
	}
	return resp, nil
}

// ChainsRegistry returns the registration for a given list of chains
func (k Keeper) ChainsRegistry(c context.Context, req *types.QueryChainsRegistryRequest) (*types.QueryChainsRegistryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	// return if no chain IDs are provided
	if len(req.ChainIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "chain IDs cannot be empty")
	}

	// return if chain IDs exceed the limit
	if len(req.ChainIds) > maxQueryChainsRegistryLimit {
		return nil, status.Errorf(codes.InvalidArgument, "cannot query more than %d chains", maxQueryChainsRegistryLimit)
	}

	// return if chain IDs contain duplicates or empty strings
	if err := bbn.CheckForDuplicatesAndEmptyStrings(req.ChainIds); err != nil {
		return nil, status.Error(codes.InvalidArgument, types.ErrInvalidChainIDs.Wrap(err.Error()).Error())
	}

	ctx := sdk.UnwrapSDKContext(c)
	var chainsRegister []*types.ChainRegister
	for _, chainID := range req.ChainIds {
		chainRegister, err := k.GetChainRegister(ctx, chainID)
		if err != nil {
			return nil, err
		}

		chainsRegister = append(chainsRegister, chainRegister)
	}

	resp := &types.QueryChainsRegistryResponse{ChainsRegister: chainsRegister}
	return resp, nil
}

// FinalityProviders returns a paginated list of all registered finality providers for a given chain
func (k Keeper) FinalityProviders(c context.Context, req *types.QueryFinalityProvidersRequest) (*types.QueryFinalityProvidersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	store := k.finalityProviderStore(ctx, req.ChainId)

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
	fp, err := k.GetFinalityProvider(ctx, req.ChainId, fpPK)
	if err != nil {
		return nil, err
	}

	fpResp := types.NewFinalityProviderResponse(fp)
	return &types.QueryFinalityProviderResponse{FinalityProvider: fpResp}, nil
}
