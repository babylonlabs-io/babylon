package keeper

import (
	"context"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) Params(c context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(c)

	return &types.QueryParamsResponse{Params: k.GetParams(ctx)}, nil
}

// FinalizedBSNsInfo returns the finalized info for a given list of BSNs
func (k Keeper) FinalizedBSNsInfo(c context.Context, req *types.QueryFinalizedBSNsInfoRequest) (*types.QueryFinalizedBSNsInfoResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	// return if no BSN IDs are provided
	if len(req.ConsumerIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "BSN ID cannot be empty")
	}

	// return if BSN IDs contain duplicates or empty strings
	if err := bbntypes.CheckForDuplicatesAndEmptyStrings(req.ConsumerIds); err != nil {
		return nil, status.Error(codes.InvalidArgument, types.ErrInvalidConsumerIDs.Wrap(err.Error()).Error())
	}

	ctx := sdk.UnwrapSDKContext(c)
	resp := &types.QueryFinalizedBSNsInfoResponse{FinalizedBsnsData: []*types.FinalizedBSNData{}}

	// find the last finalised epoch
	lastFinalizedEpoch := k.GetLastFinalizedEpoch(ctx)

	// TODO: paginate this for loop
	for _, ConsumerId := range req.ConsumerIds {
		// Validate that the BSN is registered
		if !k.HasConsumer(ctx, ConsumerId) {
			return nil, status.Error(codes.InvalidArgument, types.ErrConsumerInfoNotFound.Wrapf("BSN ID %s is not registered", ConsumerId).Error())
		}

		data := &types.FinalizedBSNData{ConsumerId: ConsumerId}

		// if no finalized header exists for this BSN in the last finalised epoch, return with empty fields
		if !k.FinalizedHeaderExists(ctx, ConsumerId, lastFinalizedEpoch) {
			resp.FinalizedBsnsData = append(resp.FinalizedBsnsData, data)
			continue
		}

		// get the finalized header for this BSN in the last finalised epoch
		finalizedHeader, err := k.GetFinalizedHeader(ctx, ConsumerId, lastFinalizedEpoch)
		if err != nil {
			return nil, err
		}

		// set finalizedEpoch as the earliest epoch that snapshots this header.
		// it's possible that the header's epoch is way before the last finalised epoch
		// e.g., when there is no relayer for many epochs
		// NOTE: if an epoch is finalised then all of its previous epochs are also finalised
		finalizedEpoch := lastFinalizedEpoch
		if finalizedHeader.Header.BabylonEpoch < finalizedEpoch {
			finalizedEpoch = finalizedHeader.Header.BabylonEpoch
		}

		data.LatestFinalizedHeader = finalizedHeader.Header

		// find the epoch metadata of the finalised epoch
		data.EpochInfo, err = k.epochingKeeper.GetHistoricalEpoch(ctx, finalizedEpoch)
		if err != nil {
			return nil, err
		}

		rawCheckpoint, err := k.checkpointingKeeper.GetRawCheckpoint(ctx, finalizedEpoch)
		if err != nil {
			return nil, err
		}

		data.RawCheckpoint = rawCheckpoint.Ckpt

		// find the raw checkpoint and the best submission key for the finalised epoch
		_, data.BtcSubmissionKey, err = k.btccKeeper.GetBestSubmission(ctx, finalizedEpoch)
		if err != nil {
			return nil, err
		}

		// generate all proofs
		if req.Prove {
			data.Proof, err = k.proveFinalizedBSN(ctx, finalizedHeader.Header, data.EpochInfo, data.BtcSubmissionKey)
			if err != nil {
				return nil, err
			}
		}

		resp.FinalizedBsnsData = append(resp.FinalizedBsnsData, data)
	}

	return resp, nil
}
