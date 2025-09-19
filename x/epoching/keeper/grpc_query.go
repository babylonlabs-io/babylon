package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/math"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Querier is used as Keeper will have duplicate methods if used directly, and gRPC names take precedence over keeper
type Querier struct {
	Keeper
}

var _ types.QueryServer = Querier{}

func (k Keeper) Params(c context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(c)

	return &types.QueryParamsResponse{Params: k.GetParams(ctx)}, nil
}

// CurrentEpoch handles the QueryCurrentEpochRequest query
func (k Keeper) CurrentEpoch(c context.Context, req *types.QueryCurrentEpochRequest) (*types.QueryCurrentEpochResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	epoch := k.GetEpoch(ctx)
	resp := &types.QueryCurrentEpochResponse{
		CurrentEpoch:  epoch.EpochNumber,
		EpochBoundary: epoch.GetLastBlockHeight(),
	}
	return resp, nil
}

// EpochInfo handles the QueryEpochInfoRequest query
func (k Keeper) EpochInfo(c context.Context, req *types.QueryEpochInfoRequest) (*types.QueryEpochInfoResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	epoch, err := k.GetHistoricalEpoch(ctx, req.EpochNum)
	if err != nil {
		return nil, err
	}

	return &types.QueryEpochInfoResponse{
		Epoch: epoch.ToResponse(),
	}, nil
}

// EpochsInfo handles the QueryEpochsInfoRequest query
func (k Keeper) EpochsInfo(c context.Context, req *types.QueryEpochsInfoRequest) (*types.QueryEpochsInfoResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	epochInfoStore := k.epochInfoStore(ctx)
	epochs := []*types.EpochResponse{}
	pageRes, err := query.Paginate(epochInfoStore, req.Pagination, func(key, value []byte) error {
		// unmarshal to epoch metadata
		var epoch types.Epoch
		if err := k.cdc.Unmarshal(value, &epoch); err != nil {
			return err
		}
		// append to epochs list
		epochs = append(epochs, epoch.ToResponse())
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryEpochsInfoResponse{
		Epochs:     epochs,
		Pagination: pageRes,
	}, nil
}

// EpochMsgs handles the QueryEpochMsgsRequest query
func (k Keeper) EpochMsgs(c context.Context, req *types.QueryEpochMsgsRequest) (*types.QueryEpochMsgsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	epoch := k.GetEpoch(ctx)
	if epoch.EpochNumber < req.EpochNum {
		return nil, types.ErrUnknownEpochNumber
	}

	if req.EpochNum == 0 {
		req.EpochNum = epoch.EpochNumber
	}

	var msgs []*types.QueuedMessageResponse
	epochMsgsStore := k.msgQueueStore(ctx, req.EpochNum)

	// handle pagination
	// TODO (non-urgent): the epoch might end between pagination requests, leading inconsistent results by the time someone gets to the end. Possible fixes:
	// - We could add the epoch number to the query, and return nothing if the current epoch number is different. But it's a bit of pain to have to set it and not know why there's no result.
	// - We could not reset the key to 0 when the queue is cleared, and just keep incrementing the ID forever. That way when the next query comes, it might skip some records that have been deleted, then resume from the next available record which has a higher key than the value in the pagination data structure.
	// - We can do nothing, in which case some records that have been inserted after the delete might be skipped because their keys are lower than the pagionation state.
	pageRes, err := query.Paginate(epochMsgsStore, req.Pagination, func(key, value []byte) error {
		// unmarshal to queuedMsg
		var sdkMsg sdk.Msg
		if err := k.cdc.UnmarshalInterface(value, &sdkMsg); err != nil {
			return err
		}
		queuedMsg, ok := sdkMsg.(*types.QueuedMessage)
		if !ok {
			return errors.New("invalid queue message")
		}
		// append to msgs
		msgs = append(msgs, queuedMsg.ToResponse())
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryEpochMsgsResponse{
		Msgs:       msgs,
		Pagination: pageRes,
	}, nil
}

// ValidatorLifecycle handles the QueryValidatorLifecycleRequest query
// TODO: test this API
func (k Keeper) ValidatorLifecycle(c context.Context, req *types.QueryValidatorLifecycleRequest) (*types.QueryValidatorLifecycleResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	valAddr, err := sdk.ValAddressFromBech32(req.ValAddr)
	if err != nil {
		return nil, err
	}
	lc := k.GetValLifecycle(ctx, valAddr)
	return &types.QueryValidatorLifecycleResponse{
		ValAddr: lc.ValAddr,
		ValLife: types.NewValsetUpdateResponses(lc.ValLife),
	}, nil
}

// DelegationLifecycle handles the QueryDelegationLifecycleRequest query
// TODO: test this API
func (k Keeper) DelegationLifecycle(c context.Context, req *types.QueryDelegationLifecycleRequest) (*types.QueryDelegationLifecycleResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	delAddr, err := sdk.AccAddressFromBech32(req.DelAddr)
	if err != nil {
		return nil, err
	}
	lc := k.GetDelegationLifecycle(ctx, delAddr)
	return &types.QueryDelegationLifecycleResponse{
		DelLife: lc,
	}, nil
}

func (k Keeper) EpochValSet(c context.Context, req *types.QueryEpochValSetRequest) (*types.QueryEpochValSetResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	epoch := k.GetEpoch(ctx)
	if epoch.EpochNumber < req.EpochNum {
		return nil, types.ErrUnknownEpochNumber
	}

	totalVotingPower := k.GetTotalVotingPower(ctx, epoch.EpochNumber)

	vals := []*types.Validator{}
	epochValSetStore := k.valSetStore(ctx, epoch.EpochNumber)
	pageRes, err := query.Paginate(epochValSetStore, req.Pagination, func(key, value []byte) error {
		// Here key is the validator's ValAddress, and value is the voting power
		var power math.Int
		if err := power.Unmarshal(value); err != nil {
			panic(errorsmod.Wrap(types.ErrUnmarshal, err.Error())) // this only happens upon a programming error
		}
		val := types.Validator{
			Addr:  key,
			Power: power.Int64(),
		}
		// append to msgs
		vals = append(vals, &val)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &types.QueryEpochValSetResponse{
		Validators:       vals,
		TotalVotingPower: totalVotingPower,
		Pagination:       pageRes,
	}
	return resp, nil
}
