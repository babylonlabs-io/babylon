package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// UpdateParams updates the params
func (ms msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.authority != req.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, req.Authority)
	}
	if err := req.Params.Validate(); err != nil {
		return nil, govtypes.ErrInvalidProposalMsg.Wrapf("invalid parameter: %v", err)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := ms.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

// WithdrawReward withdraws the reward of a given stakeholder
func (ms msgServer) WithdrawReward(goCtx context.Context, req *types.MsgWithdrawReward) (*types.MsgWithdrawRewardResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// get stakeholder type and address
	sType, err := types.NewStakeHolderTypeFromString(req.Type)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := ms.sendAllBtcDelegationTypeToRewardsGauge(ctx, sType, addr); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// withdraw reward, i.e., send withdrawable reward to the stakeholder address and clear the reward gauge
	withdrawnCoins, err := ms.withdrawReward(ctx, sType, addr)
	if err != nil {
		return nil, err
	}

	// all good
	return &types.MsgWithdrawRewardResponse{
		Coins: withdrawnCoins,
	}, nil
}

func (ms msgServer) SetWithdrawAddress(ctx context.Context, msg *types.MsgSetWithdrawAddress) (*types.MsgSetWithdrawAddressResponse, error) {
	delegatorAddress, err := sdk.AccAddressFromBech32(msg.DelegatorAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	withdrawAddress, err := sdk.AccAddressFromBech32(msg.WithdrawAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// make sure withdrawAddress it is not blocked address
	if ms.bankKeeper.BlockedAddr(withdrawAddress) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to receive external funds", withdrawAddress)
	}

	err = ms.SetWithdrawAddr(ctx, delegatorAddress, withdrawAddress)
	if err != nil {
		return nil, err
	}

	return &types.MsgSetWithdrawAddressResponse{}, nil
}
