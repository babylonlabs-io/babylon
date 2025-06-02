package keeper

import (
	"context"
	"encoding/hex"

	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/jinzhu/copier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) BlsPublicKeyList(c context.Context, req *types.QueryBlsPublicKeyListRequest) (*types.QueryBlsPublicKeyListResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	sdkCtx := sdk.UnwrapSDKContext(c)
	valBLSKeys, err := k.GetBLSPubKeySet(sdkCtx, req.EpochNum)
	if err != nil {
		return nil, err
	}

	if req.Pagination == nil {
		return &types.QueryBlsPublicKeyListResponse{
			ValidatorWithBlsKeys: convertToBlsPublicKeyListResponse(valBLSKeys),
		}, nil
	}

	total := uint64(len(valBLSKeys))
	start := req.Pagination.Offset
	if start > total-1 {
		return nil, status.Error(codes.InvalidArgument, "pagination offset out of range")
	}
	var end uint64
	if req.Pagination.Limit == 0 {
		end = total
	} else {
		end = req.Pagination.Limit + start
	}
	if end > total {
		end = total
	}
	var copiedValBLSKeys []*types.ValidatorWithBlsKey
	err = copier.Copy(&copiedValBLSKeys, valBLSKeys[start:end])
	if err != nil {
		return nil, err
	}

	return &types.QueryBlsPublicKeyListResponse{
		ValidatorWithBlsKeys: convertToBlsPublicKeyListResponse(copiedValBLSKeys),
	}, nil
}

func convertToBlsPublicKeyListResponse(valBLSKeys []*types.ValidatorWithBlsKey) []*types.BlsPublicKeyListResponse {
	blsPublicKeyListResponse := make([]*types.BlsPublicKeyListResponse, len(valBLSKeys))

	for i, valBlsKey := range valBLSKeys {
		blsPublicKeyListResponse[i] = &types.BlsPublicKeyListResponse{
			ValidatorAddress: valBlsKey.ValidatorAddress,
			BlsPubKeyHex:     hex.EncodeToString(valBlsKey.BlsPubKey),
			VotingPower:      valBlsKey.VotingPower,
		}
	}
	return blsPublicKeyListResponse
}
