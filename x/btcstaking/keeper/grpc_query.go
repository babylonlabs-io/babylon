package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

var _ types.QueryServer = Keeper{}

// FinalityProviders returns a paginated list of all Babylon maintained finality providers
func (k Keeper) FinalityProviders(c context.Context, req *types.QueryFinalityProvidersRequest) (*types.QueryFinalityProvidersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	store := k.finalityProviderStore(ctx)
	currBlockHeight := uint64(ctx.BlockHeight())

	var fpResp []*types.FinalityProviderResponse
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		var fp types.FinalityProvider
		if err := fp.Unmarshal(value); err != nil {
			return err
		}

		resp := types.NewFinalityProviderResponse(&fp, currBlockHeight)
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
	fp, err := k.GetFinalityProvider(ctx, key)
	if err != nil {
		return nil, err
	}

	currBlockHeight := uint64(ctx.BlockHeight())
	fpResp := types.NewFinalityProviderResponse(fp, currBlockHeight)
	return &types.QueryFinalityProviderResponse{FinalityProvider: fpResp}, nil
}

// BTCDelegations returns all BTC delegations under a given status
func (k Keeper) BTCDelegations(ctx context.Context, req *types.QueryBTCDelegationsRequest) (*types.QueryBTCDelegationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	covenantQuorum := k.GetParams(ctx).CovenantQuorum

	// get current BTC height
	btcTipHeight := k.btclcKeeper.GetTipInfo(ctx).Height

	store := k.btcDelegationStore(ctx)
	var btcDels []*types.BTCDelegationResponse
	pageRes, err := query.FilteredPaginate(store, req.Pagination, func(_ []byte, value []byte, accumulate bool) (bool, error) {
		var btcDel types.BTCDelegation
		k.cdc.MustUnmarshal(value, &btcDel)

		// hit if the queried status is ANY or matches the BTC delegation status
		status := btcDel.GetStatus(btcTipHeight, covenantQuorum)
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

	fpPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	btcDelStore := k.btcDelegatorFpStore(sdkCtx, fpPK)

	btcHeight := k.btclcKeeper.GetTipInfo(ctx).Height
	covenantQuorum := k.GetParams(ctx).CovenantQuorum

	btcDels := []*types.BTCDelegatorDelegationsResponse{}
	pageRes, err := query.Paginate(btcDelStore, req.Pagination, func(key, value []byte) error {
		delBTCPK, err := bbn.NewBIP340PubKey(key)
		if err != nil {
			return err
		}

		curBTCDels := k.getBTCDelegatorDelegations(sdkCtx, fpPK, delBTCPK)

		btcDelsResp := make([]*types.BTCDelegationResponse, len(curBTCDels.Dels))
		for i, btcDel := range curBTCDels.Dels {
			status := btcDel.GetStatus(
				btcHeight,
				covenantQuorum,
			)
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

	// decode staking tx hash
	stakingTxHash, err := chainhash.NewHashFromStr(req.StakingTxHashHex)
	if err != nil {
		return nil, err
	}

	// find BTC delegation
	btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
	if btcDel == nil {
		return nil, types.ErrBTCDelegationNotFound
	}

	status := btcDel.GetStatus(
		k.btclcKeeper.GetTipInfo(ctx).Height,
		k.GetParams(ctx).CovenantQuorum,
	)

	return &types.QueryBTCDelegationResponse{
		BtcDelegation: types.NewBTCDelegationResponse(btcDel, status),
	}, nil
}
