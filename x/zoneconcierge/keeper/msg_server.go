package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonchain/babylon/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
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
	if ms.IsChainRegistered(ctx, req.ChainId) {
		return nil, types.ErrChainAlreadyRegistered
	}
	ms.SetChainRegister(ctx, &chainRegister)

	return &types.MsgRegisterChainResponse{}, nil
}
