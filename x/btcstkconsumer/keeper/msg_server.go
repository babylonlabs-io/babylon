package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

// RegisterConsumer registers a CZ consumer
func (ms msgServer) RegisterConsumer(goCtx context.Context, req *types.MsgRegisterConsumer) (*types.MsgRegisterConsumerResponse, error) {
	consumerRegister := &types.ConsumerRegister{
		ConsumerId:          req.ConsumerId,
		ConsumerName:        req.ConsumerName,
		ConsumerDescription: req.ConsumerDescription,
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	err := ms.Keeper.RegisterConsumer(ctx, consumerRegister)
	if err != nil {
		return nil, err
	}

	return &types.MsgRegisterConsumerResponse{}, nil
}
