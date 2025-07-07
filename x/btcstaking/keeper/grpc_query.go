package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

var _ types.QueryServer = Keeper{}

// FinalityProviders returns a paginated list of all Babylon maintained finality providers
func (k Keeper) FinalityProviders(c context.Context, req *types.QueryFinalityProvidersRequest) (*types.QueryFinalityProvidersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	indexStore := k.finalityProviderBsnIndexStore(ctx)
	// Default to Babylon BSN id if empty
	bsnId := req.BsnId
	if bsnId == "" {
		bsnId = ctx.ChainID()
	}
	bsnPrefix := types.BuildBsnIndexPrefix(bsnId)
	prefixStore := prefix.NewStore(indexStore, bsnPrefix)

	var fpResp []*types.FinalityProviderResponse
	currBlockHeight := uint64(ctx.BlockHeight())
	pageRes, err := query.Paginate(prefixStore, req.Pagination, func(key, _ []byte) error {
		// Get full FP from primary storage
		fp, err := k.GetFinalityProvider(ctx, key)
		if err != nil {
			return err
		}
		resp := types.NewFinalityProviderResponse(fp, currBlockHeight)
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

	key, err := fpPK.Marshal()
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c)
	currBlockHeight := uint64(ctx.BlockHeight())

	fp, err := k.GetFinalityProvider(ctx, key)
	if err != nil {
		return nil, err
	}

	fpResp := types.NewFinalityProviderResponse(fp, currBlockHeight)
	return &types.QueryFinalityProviderResponse{FinalityProvider: fpResp}, nil
}

// BTCDelegations returns all BTC delegations under a given status
func (k Keeper) BTCDelegations(ctx context.Context, req *types.QueryBTCDelegationsRequest) (*types.QueryBTCDelegationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	// get current BTC height
	btcTipHeight := k.btclcKeeper.GetTipInfo(ctx).Height

	store := k.btcDelegationStore(ctx)
	var btcDels []*types.BTCDelegationResponse
	pageRes, err := query.FilteredPaginate(store, req.Pagination, func(_ []byte, value []byte, accumulate bool) (bool, error) {
		var btcDel types.BTCDelegation
		k.cdc.MustUnmarshal(value, &btcDel)

		params := k.GetParamsByVersion(ctx, btcDel.ParamsVersion)

		// hit if the queried status is ANY or matches the BTC delegation status
		status, err := k.BtcDelStatus(ctx, &btcDel, params.CovenantQuorum, btcTipHeight)
		if err != nil {
			return true, err
		}
		if req.Status == types.BTCDelegationStatus_ANY || status == req.Status {
			if accumulate {
				resp := types.NewBTCDelegationResponse(&btcDel, status)
				btcDels = append(btcDels, resp)
			}
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryBTCDelegationsResponse{
		BtcDelegations: btcDels,
		Pagination:     pageRes,
	}, nil
}

// FinalityProviderDelegations returns all the delegations of the provided finality provider filtered by the provided status.
func (k Keeper) FinalityProviderDelegations(ctx context.Context, req *types.QueryFinalityProviderDelegationsRequest) (*types.QueryFinalityProviderDelegationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if len(req.FpBtcPkHex) == 0 {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "finality provider BTC public key cannot be empty")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	btcHeight := k.btclcKeeper.GetTipInfo(ctx).Height

	fpPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, err
	}

	var (
		btcDels []*types.BTCDelegatorDelegationsResponse
		pageRes *query.PageResponse
	)
	if !k.HasFinalityProvider(ctx, *fpPK) {
		return nil, types.ErrFpNotFound
	}

	btcDelStore := k.btcDelegatorFpStore(sdkCtx, fpPK)
	pageRes, err = query.Paginate(btcDelStore, req.Pagination, func(key, value []byte) error {
		delBTCPK, err := bbn.NewBIP340PubKey(key)
		if err != nil {
			return err
		}

		curBTCDels := k.getBTCDelegatorDelegations(sdkCtx, fpPK, delBTCPK)

		btcDelsResp := make([]*types.BTCDelegationResponse, len(curBTCDels.Dels))
		for i, btcDel := range curBTCDels.Dels {
			params := k.GetParamsByVersion(sdkCtx, btcDel.ParamsVersion)

			status, err := k.BtcDelStatus(
				ctx,
				btcDel,
				params.CovenantQuorum,
				btcHeight,
			)
			if err != nil {
				return err
			}
			btcDelsResp[i] = types.NewBTCDelegationResponse(btcDel, status)
		}

		btcDels = append(btcDels, &types.BTCDelegatorDelegationsResponse{
			Dels: btcDelsResp,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryFinalityProviderDelegationsResponse{BtcDelegatorDelegations: btcDels, Pagination: pageRes}, nil
}

// BTCDelegation returns existing btc delegation by staking tx hash
func (k Keeper) BTCDelegation(ctx context.Context, req *types.QueryBTCDelegationRequest) (*types.QueryBTCDelegationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	delInfo, err := k.getBTCDelWithParams(ctx, req.StakingTxHashHex)
	if err != nil {
		return nil, err
	}

	status, _, err := k.BtcDelStatusWithTip(ctx, delInfo)
	if err != nil {
		return nil, err
	}
	return &types.QueryBTCDelegationResponse{
		BtcDelegation: types.NewBTCDelegationResponse(delInfo.Delegation, status),
	}, nil
}

// LargestBtcReOrg implements types.QueryServer.
func (k Keeper) LargestBtcReOrg(ctx context.Context, _ *types.QueryLargestBtcReOrgRequest) (*types.QueryLargestBtcReOrgResponse, error) {
	largestBtcReorg, err := k.LargestBtcReorg.Get(ctx)
	if err != nil {
		return nil, types.ErrLargestBtcReorgNotFound
	}

	return &types.QueryLargestBtcReOrgResponse{
		BlockDiff:    largestBtcReorg.BlockDiff,
		RollbackFrom: largestBtcReorg.RollbackFrom.ToResponse(),
		RollbackTo:   largestBtcReorg.RollbackTo.ToResponse(),
	}, nil
}

// ParamsVersions iterates over all the versioned parameters in the store.
func (k Keeper) ParamsVersions(c context.Context, req *types.QueryParamsVersionsRequest) (*types.QueryParamsVersionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	store := k.paramsStore(ctx)

	var resp []types.StoredParams
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		var sp types.StoredParams
		if err := sp.Unmarshal(value); err != nil {
			return err
		}

		resp = append(resp, sp)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsVersionsResponse{Params: resp, Pagination: pageRes}, nil
}
