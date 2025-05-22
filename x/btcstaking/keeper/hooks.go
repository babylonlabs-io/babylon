package keeper

import (
	"context"

	ltypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
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

	if err := h.k.SetLargestBtcReorg(ctx, types.NewLargestBtcReOrg(rollbackFrom, rollbackTo)); err != nil {
		h.k.Logger(ctx).Error("failed to set largest BTC reorg", zap.Error(err))
	}
}

// AfterBTCHeaderInserted implements HandledHooks.
func (h Hooks) AfterBTCHeaderInserted(_ context.Context, _ *ltypes.BTCHeaderInfo) {
}

// AfterBTCRollForward implements HandledHooks.
func (h Hooks) AfterBTCRollForward(_ context.Context, _ *ltypes.BTCHeaderInfo) {
}
