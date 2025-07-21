package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/runtime"
)

// GetBSNBTCState gets the unified BTC state for a specific BSN
func (k Keeper) GetBSNBTCState(ctx context.Context, consumerID string) *types.BSNBTCState {
	store := k.bsnBTCStateStore(ctx)
	stateBytes := store.Get([]byte(consumerID))
	if len(stateBytes) == 0 {
		return nil
	}
	var state types.BSNBTCState
	k.cdc.MustUnmarshal(stateBytes, &state)
	return &state
}

// SetBSNBTCState sets the unified BTC state for a specific BSN
func (k Keeper) SetBSNBTCState(ctx context.Context, consumerID string, state *types.BSNBTCState) {
	store := k.bsnBTCStateStore(ctx)
	stateBytes := k.cdc.MustMarshal(state)
	store.Set([]byte(consumerID), stateBytes)
}

// GetBSNBaseBTCHeader gets the base BTC header for a specific BSN
func (k Keeper) GetBSNBaseBTCHeader(ctx context.Context, consumerID string) *btclctypes.BTCHeaderInfo {
	state := k.GetBSNBTCState(ctx, consumerID)
	if state == nil || state.BaseHeader == nil {
		return nil
	}
	return state.BaseHeader
}

// SetBSNBaseBTCHeader sets the base BTC header for a specific BSN
func (k Keeper) SetBSNBaseBTCHeader(ctx context.Context, consumerID string, header *btclctypes.BTCHeaderInfo) {
	state := k.GetBSNBTCState(ctx, consumerID)
	if state == nil {
		state = &types.BSNBTCState{}
	}
	state.BaseHeader = header
	k.SetBSNBTCState(ctx, consumerID, state)
}

// GetBSNLastSentSegment gets the last sent segment for a specific BSN
func (k Keeper) GetBSNLastSentSegment(ctx context.Context, consumerID string) *types.BTCChainSegment {
	state := k.GetBSNBTCState(ctx, consumerID)
	if state == nil || state.LastSentSegment == nil {
		return nil
	}
	return state.LastSentSegment
}

// SetBSNLastSentSegment sets the last sent segment for a specific BSN
func (k Keeper) SetBSNLastSentSegment(ctx context.Context, consumerID string, segment *types.BTCChainSegment) {
	state := k.GetBSNBTCState(ctx, consumerID)
	if state == nil {
		state = &types.BSNBTCState{}
	}
	state.LastSentSegment = segment
	k.SetBSNBTCState(ctx, consumerID, state)
}

// bsnBTCStateStore stores the unified BTC state for each BSN
// prefix: BSNBTCStateKey
// key: consumerID
// value: BSNBTCState
func (k Keeper) bsnBTCStateStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.BSNBTCStateKey)
}

// InitializeBSNBTCState initializes the BTC state for a BSN
// This is called when a new IBC channel is created to set the starting point for BTC header synchronization
// The base header is set to the current BTC tip
func (k Keeper) InitializeBSNBTCState(ctx context.Context, consumerID string) error {
	// Check if BSN state already exists
	if existingState := k.GetBSNBTCState(ctx, consumerID); existingState != nil {
		// BSN state already exists, no need to initialize
		return nil
	}

	// Get the current tip of the BTC light client
	tipInfo := k.btclcKeeper.GetTipInfo(ctx)
	if tipInfo == nil {
		return fmt.Errorf("BTC light client tip info is nil")
	}

	// Initialize the state with the current tip as the base header
	state := &types.BSNBTCState{
		BaseHeader:      tipInfo,
		LastSentSegment: nil, // No headers sent yet
	}

	k.SetBSNBTCState(ctx, consumerID, state)
	return nil
}
