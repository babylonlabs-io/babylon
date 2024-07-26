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

func (k Keeper) triggerRollBack(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	// Trigger AfterBTCRollBack hook
	k.AfterBTCRollBack(ctx, headerInfo)
	// Emit BTCRollBack event
	k.emitTypedEventWithLog(ctx, &types.EventBTCRollBack{Header: headerInfo})
}

func (k Keeper) triggerRollForward(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	// Trigger AfterBTCRollForward hook
	k.AfterBTCRollForward(ctx, headerInfo)
	// Emit BTCRollForward event
	k.emitTypedEventWithLog(ctx, &types.EventBTCRollForward{Header: headerInfo})
}
