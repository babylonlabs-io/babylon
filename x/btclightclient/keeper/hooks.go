package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
)

// Implements BTCLightClientHooks interface
var _ types.BTCLightClientHooks = Keeper{}

// AfterBTCHeaderInserted - call hook if registered
func (k Keeper) AfterBTCHeaderInserted(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	if k.hooks != nil {
		k.hooks.AfterBTCHeaderInserted(ctx, headerInfo)
	}
}

// AfterBTCRollBack - call hook if registered
func (k Keeper) AfterBTCRollBack(ctx context.Context, rollbackFrom, rollbackTo *types.BTCHeaderInfo) {
	if k.hooks != nil {
		k.hooks.AfterBTCRollBack(ctx, rollbackFrom, rollbackTo)
	}
}

// AfterBTCRollForward - call hook if registered
func (k Keeper) AfterBTCRollForward(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	if k.hooks != nil {
		k.hooks.AfterBTCRollForward(ctx, headerInfo)
	}
}
