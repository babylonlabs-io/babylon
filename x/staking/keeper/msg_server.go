package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/types"

	epochingkeeper "github.com/babylonlabs-io/babylon/x/epoching/keeper"
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
)

type msgServer struct {
	types.MsgServer

	epochK *epochingkeeper.Keeper
}

// NewMsgServerImpl returns an implementation of the staking MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(k *keeper.Keeper, epochK *epochingkeeper.Keeper) types.MsgServer {
	return &msgServer{
		MsgServer: keeper.NewMsgServerImpl(k),
		epochK:    epochK,
	}
}

// Delegate defines a method for performing a delegation of coins from a delegator to a validator
func (ms msgServer) Delegate(goCtx context.Context, msg *types.MsgDelegate) (*types.MsgDelegateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if ctx.Value(epochingtypes.CtxKeyUnwrapMsgServer).(bool) {
		return ms.MsgServer.Delegate(goCtx, msg)
	}

	_, err := ms.epochK.WrappedDelegate(ctx, epochingtypes.NewMsgWrappedDelegate(msg))
	if err != nil {
		return nil, err
	}
	return &types.MsgDelegateResponse{}, nil
}
