package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/btclightclient/types"
)

func (k Keeper) triggerHeaderInserted(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	// Trigger AfterBTCHeaderInserted hook
	k.AfterBTCHeaderInserted(ctx, headerInfo)
	// Emit HeaderInserted event
	k.emitTypedEventWithLog(ctx, &types.EventBTCHeaderInserted{Header: headerInfo})
}

// triggerRollBack calls the hook and emits an event, the rollbackFrom is the latest tip
// prior rollbackTo best block is sent
func (k Keeper) triggerRollBack(ctx context.Context, rollbackFrom, rollbackTo *types.BTCHeaderInfo) {
	// Trigger AfterBTCRollBack hook
	k.AfterBTCRollBack(ctx, rollbackFrom, rollbackTo)
	// Emit BTCRollBack event
	k.emitTypedEventWithLog(ctx, &types.EventBTCRollBack{Header: rollbackTo})
}

func (k Keeper) triggerRollForward(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	// Trigger AfterBTCRollForward hook
	k.AfterBTCRollForward(ctx, headerInfo)
	// Emit BTCRollForward event
	k.emitTypedEventWithLog(ctx, &types.EventBTCRollForward{Header: headerInfo})
}
