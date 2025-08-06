package keeper

import (
	"context"
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
