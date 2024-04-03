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

// RegisterChain registers a CZ chain
func (ms msgServer) RegisterChain(goCtx context.Context, req *types.MsgRegisterChain) (*types.MsgRegisterChainResponse, error) {
	// Create ChanRegister from MsgRegisterChain
	chainRegister := types.ChainRegister{
		ChainId:          req.ChainId,
		ChainName:        req.ChainName,
		ChainDescription: req.ChainDescription,
	}

	if err := chainRegister.Validate(); err != nil {
		return nil, types.ErrInvalidChainRegister.Wrapf("invalid chain: %v", err)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if ms.IsConsumerChainRegistered(ctx, req.ChainId) {
		return nil, types.ErrChainAlreadyRegistered
	}
	ms.SetChainRegister(ctx, &chainRegister)

	return &types.MsgRegisterChainResponse{}, nil
}
