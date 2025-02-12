package keeper

import (
	"context"

	ltypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
)

// HandledHooks Helper interface to ensure Hooks implements
// the btclightclient hook
type HandledHooks interface {
	ltypes.BTCLightClientHooks
}

type Hooks struct {
	k Keeper
}

var _ HandledHooks = Hooks{}

func (k Keeper) Hooks() Hooks { return Hooks{k} }

// AfterBTCRollBack updates the Largest BTC reorg if it is higher than the current one
// to later verify in EndBlocker.
func (h Hooks) AfterBTCRollBack(goCtx context.Context, rollbackFrom, rollbackTo *ltypes.BTCHeaderInfo) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// sanity checks for rollback values
	if rollbackFrom == nil {
		h.k.Logger(ctx).Debug("BTC rollback without rollbackFrom")
		return
	}

	if rollbackTo == nil {
		h.k.Logger(ctx).Debug("BTC rollback without rollbackTo")
		return
	}

	// should verify that the BTC height it is rolling back is lower than the latest tip
	if rollbackTo.Height >= rollbackFrom.Height {
		h.k.Logger(ctx).Warn(
			"BTC rollback with rollback height 'To' higher or equal than 'From'",
			"from_height", rollbackFrom.Height,
			"to_height", rollbackTo.Height,
		)
		return
	}

	newReorg := rollbackFrom.Height - rollbackTo.Height
	if err := h.k.SetLargestBtcReorg(ctx, newReorg); err != nil {
		h.k.Logger(ctx).Error("failed to set largest BTC reorg", zap.Error(err))
	}
}

// AfterBTCHeaderInserted implements HandledHooks.
func (h Hooks) AfterBTCHeaderInserted(_ context.Context, _ *ltypes.BTCHeaderInfo) {
}

// AfterBTCRollForward implements HandledHooks.
func (h Hooks) AfterBTCRollForward(_ context.Context, _ *ltypes.BTCHeaderInfo) {
}
