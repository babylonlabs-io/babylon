package keeper

import (
	"context"

	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
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
	// Create ChanRegister from MsgRegisterConsumer
	consumerRegister := types.ConsumerRegister{
		ConsumerId:          req.ConsumerId,
		ConsumerName:        req.ConsumerName,
		ConsumerDescription: req.ConsumerDescription,
	}

	if err := consumerRegister.Validate(); err != nil {
		return nil, types.ErrInvalidConsumerRegister.Wrapf("invalid consumer: %v", err)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if ms.IsConsumerRegistered(ctx, req.ConsumerId) {
		return nil, types.ErrConsumerAlreadyRegistered
	}
	ms.SetConsumerRegister(ctx, &consumerRegister)

	return &types.MsgRegisterConsumerResponse{}, nil
}
