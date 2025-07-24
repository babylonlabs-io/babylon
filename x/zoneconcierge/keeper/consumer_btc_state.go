package keeper

import (
	"context"
	"fmt"

	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// GetBSNBTCState gets the unified BTC state for a specific BSN
func (k Keeper) GetBSNBTCState(ctx context.Context, consumerID string) *types.BSNBTCState {
	state, err := k.BSNBTCState.Get(ctx, consumerID)
	if err != nil {
		return nil
	}
	return &state
}

// SetBSNBTCState sets the unified BTC state for a specific BSN
func (k Keeper) SetBSNBTCState(ctx context.Context, consumerID string, state *types.BSNBTCState) error {
	return k.BSNBTCState.Set(ctx, consumerID, *state)
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
	if err := k.SetBSNBTCState(ctx, consumerID, state); err != nil {
		panic(err)
	}
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
	if err := k.SetBSNBTCState(ctx, consumerID, state); err != nil {
		panic(err)
	}
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

	return k.SetBSNBTCState(ctx, consumerID, state)
}
