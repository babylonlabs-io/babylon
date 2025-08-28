package keeper

import (
	"context"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// Implements btclightclient.BTCLightClientHooks interface for ZoneConcierge
var _ btclctypes.BTCLightClientHooks = (*Keeper)(nil)

// AfterBTCHeaderInserted is called after a new BTC header is inserted
// This marks that BTC headers should be broadcast in this block's EndBlocker
func (k Keeper) AfterBTCHeaderInserted(ctx context.Context, headerInfo *btclctypes.BTCHeaderInfo) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	transientStore := sdkCtx.TransientStore(k.transientKey)

	// Mark that new BTC header was inserted - this will trigger broadcasting
	transientStore.Set(types.BTCHeaderInsertedKey, []byte{1})

	k.Logger(sdkCtx).Debug("BTC header inserted, will broadcast in EndBlocker",
		"height", headerInfo.Height,
		"hash", headerInfo.Hash.String(),
	)
}

// AfterBTCRollBack is called after a BTC chain rollback (reorg)
// This marks that BTC headers should be broadcast due to reorganization
func (k Keeper) AfterBTCRollBack(ctx context.Context, rollbackFrom, rollbackTo *btclctypes.BTCHeaderInfo) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	transientStore := sdkCtx.TransientStore(k.transientKey)

	// Mark that BTC reorg occurred - this will trigger broadcasting
	transientStore.Set(types.BTCReorgOccurredKey, []byte{1})

	k.Logger(sdkCtx).Info("BTC chain rollback detected, will broadcast in EndBlocker",
		"rollback_from_height", rollbackFrom.Height,
		"rollback_to_height", rollbackTo.Height,
	)
}

// AfterBTCRollForward is called after a BTC chain roll forward
// This is typically called after rollback to rebuild the chain
func (k Keeper) AfterBTCRollForward(ctx context.Context, headerInfo *btclctypes.BTCHeaderInfo) {
	// Roll forward events typically follow rollbacks, so we don't need separate handling
	// The rollback hook already marked for broadcasting
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.Logger(sdkCtx).Debug("BTC chain roll forward",
		"height", headerInfo.Height,
	)
}

// MarkNewConsumerChannel should be called when a new consumer channel is opened
// This can be called from IBC channel creation
func (k Keeper) MarkNewConsumerChannel(ctx context.Context, consumerID string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	transientStore := sdkCtx.TransientStore(k.transientKey)

	// Mark that new consumer channel was opened - this will trigger broadcasting
	transientStore.Set(types.NewConsumerChannelKey, []byte{1})
}

// ShouldBroadcastBTCHeaders checks if BTC headers should be broadcast in this block
// Returns true if any of the trigger conditions are met:
// 1. New BTC header was inserted
// 2. BTC reorg occurred
// 3. New consumer channel was opened
func (k Keeper) ShouldBroadcastBTCHeaders(ctx context.Context) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	transientStore := sdkCtx.TransientStore(k.transientKey)

	// Check for new BTC header insertion
	if transientStore.Has(types.BTCHeaderInsertedKey) {
		k.Logger(sdkCtx).Debug("Broadcasting triggered by new BTC header")
		return true
	}

	// Check for BTC reorg
	if transientStore.Has(types.BTCReorgOccurredKey) {
		k.Logger(sdkCtx).Debug("Broadcasting triggered by BTC reorg")
		return true
	}

	// Check for new consumer channel
	if transientStore.Has(types.NewConsumerChannelKey) {
		k.Logger(sdkCtx).Debug("Broadcasting triggered by new consumer channel")
		return true
	}
	return false
}

// GetBroadcastTriggerReason returns the reason for broadcasting for logging/testing
func (k Keeper) GetBroadcastTriggerReason(ctx context.Context) string {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	transientStore := sdkCtx.TransientStore(k.transientKey)

	reasons := []string{}

	if transientStore.Has(types.BTCHeaderInsertedKey) {
		reasons = append(reasons, "new_btc_header")
	}

	if transientStore.Has(types.BTCReorgOccurredKey) {
		reasons = append(reasons, "btc_reorg")
	}

	if transientStore.Has(types.NewConsumerChannelKey) {
		reasons = append(reasons, "new_consumer_channel")
	}

	if len(reasons) == 0 {
		return "none"
	}

	return strings.Join(reasons, ",")
}
