package keeper

import (
	"context"
	"encoding/hex"

	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
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

	total := uint64(len(valBLSKeys))
	offset := req.Pagination.GetOffset()
	limit := req.Pagination.GetLimit()

	if offset > total {
		return nil, status.Errorf(codes.InvalidArgument, "pagination offset out of range: offset %d higher than total %d", offset, total)
	}

	end := offset + limit
	if limit == 0 || end > total {
		end = total
	}

	paginated := valBLSKeys[offset:end]

	return &types.QueryBlsPublicKeyListResponse{
		ValidatorWithBlsKeys: convertToBlsPublicKeyListResponse(paginated),
		Pagination: &query.PageResponse{
			Total: total,
		},
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
