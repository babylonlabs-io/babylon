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

// FinalizedChainsInfo returns the finalized info for a given list of chains
func (k Keeper) FinalizedChainsInfo(c context.Context, req *types.QueryFinalizedChainsInfoRequest) (*types.QueryFinalizedChainsInfoResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	// return if no chain IDs are provided
	if len(req.ConsumerIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "chain ID cannot be empty")
	}

	// return if chain IDs contain duplicates or empty strings
	if err := bbntypes.CheckForDuplicatesAndEmptyStrings(req.ConsumerIds); err != nil {
		return nil, status.Error(codes.InvalidArgument, types.ErrInvalidConsumerIDs.Wrap(err.Error()).Error())
	}

	ctx := sdk.UnwrapSDKContext(c)
	resp := &types.QueryFinalizedChainsInfoResponse{FinalizedChainsInfo: []*types.FinalizedChainInfo{}}

	// find the last finalised epoch
	lastFinalizedEpoch := k.GetLastFinalizedEpoch(ctx)
	// TODO: paginate this for loop
	for _, ConsumerId := range req.ConsumerIds {
		// check if chain ID is valid
		if !k.HasChainInfo(ctx, ConsumerId) {
			return nil, status.Error(codes.InvalidArgument, types.ErrChainInfoNotFound.Wrapf("chain ID %s", ConsumerId).Error())
		}

		data := &types.FinalizedChainInfo{ConsumerId: ConsumerId}

		// if the chain info is not found in the last finalised epoch, return with empty fields
		if !k.EpochChainInfoExists(ctx, ConsumerId, lastFinalizedEpoch) {
			resp.FinalizedChainsInfo = append(resp.FinalizedChainsInfo, data)
			continue
		}

		// find the chain info in the last finalised epoch
		chainInfoWithProof, err := k.GetEpochChainInfo(ctx, ConsumerId, lastFinalizedEpoch)
		if err != nil {
			return nil, err
		}
		chainInfo := chainInfoWithProof.ChainInfo

		// set finalizedEpoch as the earliest epoch that snapshots this chain info.
		// it's possible that the chain info's epoch is way before the last finalised epoch
		// e.g., when there is no relayer for many epochs
		// NOTE: if an epoch is finalised then all of its previous epochs are also finalised
		finalizedEpoch := lastFinalizedEpoch
		if chainInfo.LatestHeader.BabylonEpoch < finalizedEpoch {
			finalizedEpoch = chainInfo.LatestHeader.BabylonEpoch
		}

		data.FinalizedChainInfo = chainInfo

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
			data.Proof, err = k.proveFinalizedChainInfo(ctx, chainInfo, data.EpochInfo, data.BtcSubmissionKey)
			if err != nil {
				return nil, err
			}
		}

		resp.FinalizedChainsInfo = append(resp.FinalizedChainsInfo, data)
	}

	return resp, nil
}

func (k Keeper) FinalizedChainInfoUntilHeight(c context.Context, req *types.QueryFinalizedChainInfoUntilHeightRequest) (*types.QueryFinalizedChainInfoUntilHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if len(req.ConsumerId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "chain ID cannot be empty")
	}

	ctx := sdk.UnwrapSDKContext(c)
	resp := &types.QueryFinalizedChainInfoUntilHeightResponse{}

	// find the last finalised epoch
	lastFinalizedEpoch := k.GetLastFinalizedEpoch(ctx)
	// find the chain info in the last finalised epoch
	chainInfoWithProof, err := k.GetEpochChainInfo(ctx, req.ConsumerId, lastFinalizedEpoch)
	if err != nil {
		return nil, err
	}
	chainInfo := chainInfoWithProof.ChainInfo

	// set finalizedEpoch as the earliest epoch that snapshots this chain info.
	// it's possible that the chain info's epoch is way before the last finalised epoch
	// e.g., when there is no relayer for many epochs
	// NOTE: if an epoch is finalised then all of its previous epochs are also finalised
	finalizedEpoch := lastFinalizedEpoch
	if chainInfo.LatestHeader.BabylonEpoch < finalizedEpoch {
		finalizedEpoch = chainInfo.LatestHeader.BabylonEpoch
	}

	if chainInfo.LatestHeader.Height <= req.Height { // the requested height is after the last finalised chain info
		// find and assign the epoch metadata of the finalised epoch
		resp.EpochInfo, err = k.epochingKeeper.GetHistoricalEpoch(ctx, finalizedEpoch)
		if err != nil {
			return nil, err
		}

		rawCheckpoint, err := k.checkpointingKeeper.GetRawCheckpoint(ctx, finalizedEpoch)

		if err != nil {
			return nil, err
		}

		resp.RawCheckpoint = rawCheckpoint.Ckpt

		// find and assign the raw checkpoint and the best submission key for the finalised epoch
		_, resp.BtcSubmissionKey, err = k.btccKeeper.GetBestSubmission(ctx, finalizedEpoch)
		if err != nil {
			return nil, err
		}
	} else { // the requested height is before the last finalised chain info
		// starting from the requested height, iterate backward until a timestamped header
		closestHeader, err := k.FindClosestHeader(ctx, req.ConsumerId, req.Height)
		if err != nil {
			return nil, err
		}
		// assign the finalizedEpoch, and retrieve epoch info, raw ckpt and submission key
		finalizedEpoch = closestHeader.BabylonEpoch
		resp.EpochInfo, err = k.epochingKeeper.GetHistoricalEpoch(ctx, finalizedEpoch)
		if err != nil {
			return nil, err
		}

		rawCheckpoint, err := k.checkpointingKeeper.GetRawCheckpoint(ctx, finalizedEpoch)

		if err != nil {
			return nil, err
		}

		resp.RawCheckpoint = rawCheckpoint.Ckpt

		_, resp.BtcSubmissionKey, err = k.btccKeeper.GetBestSubmission(ctx, finalizedEpoch)
		if err != nil {
			return nil, err
		}
	}

	// if the query does not want the proofs, return here
	if !req.Prove {
		return resp, nil
	}

	// generate all proofs
	resp.Proof, err = k.proveFinalizedChainInfo(ctx, chainInfo, resp.EpochInfo, resp.BtcSubmissionKey)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
