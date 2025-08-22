package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

var _ types.MsgServer = MsgServer{}

type MsgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &MsgServer{Keeper: k}
}

// UpdateParams updates the params checking if there is a need to update the coostaker scores
func (ms MsgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.authority != req.Authority {
		return nil, govtypes.ErrInvalidSigner.Wrapf("invalid authority; expected %s, got %s", ms.authority, req.Authority)
	}
	if err := req.Params.Validate(); err != nil {
		return nil, govtypes.ErrInvalidProposalMsg.Wrapf("invalid parameter: %v", err)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	currentParams := ms.GetParams(ctx)
	if !req.Params.ScoreRatioBtcByBaby.Equal(currentParams.ScoreRatioBtcByBaby) {
		// if the score ratio continues the same, no need to iterate over all coostakers
		err := ms.UpdateAllCoostakersScore(ctx, req.Params.ScoreRatioBtcByBaby)
		if err != nil {
			return nil, govtypes.ErrInvalidProposalMsg.Wrapf("unable to update all the coostakers score: %v", err)
		}
	}

	if err := ms.SetParams(ctx, req.Params); err != nil {
		return nil, govtypes.ErrInvalidProposalMsg.Wrapf("unable to set params: %v", err)
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
