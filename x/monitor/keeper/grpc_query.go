package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/monitor/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) EndedEpochBtcHeight(c context.Context, req *types.QueryEndedEpochBtcHeightRequest) (*types.QueryEndedEpochBtcHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	btcHeight, err := k.LightclientHeightAtEpochEnd(ctx, req.EpochNum)

	if err != nil {
		return nil, err
	}

	return &types.QueryEndedEpochBtcHeightResponse{BtcLightClientHeight: btcHeight}, nil
}

func (k Keeper) ReportedCheckpointBtcHeight(c context.Context, req *types.QueryReportedCheckpointBtcHeightRequest) (*types.QueryReportedCheckpointBtcHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	btcHeight, err := k.LightclientHeightAtCheckpointReported(ctx, req.CkptHash)

	if err != nil {
		return nil, err
	}

	return &types.QueryReportedCheckpointBtcHeightResponse{BtcLightClientHeight: btcHeight}, nil
}
