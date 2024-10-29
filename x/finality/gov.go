package finality

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/babylonlabs-io/babylon/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

// NewResumeFinalityProposalHandler is a handler for governance proposal on resume finality.
func NewResumeFinalityProposalHandler(k keeper.Keeper) govtypesv1.Handler {
	return func(ctx sdk.Context, content govtypesv1.Content) error {
		switch c := content.(type) {
		case *types.ResumeFinalityProposal:
			return handleResumeFinalityProposal(ctx, k, c)

		default:
			return errorsmod.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized resume finality proposal content type: %T", c)
		}
	}
}

// handleResumeFinalityProposal is a handler for jail finality provider proposals
func handleResumeFinalityProposal(ctx sdk.Context, k keeper.Keeper, p *types.ResumeFinalityProposal) error {
	return k.HandleResumeFinalityProposal(ctx, p)
}
