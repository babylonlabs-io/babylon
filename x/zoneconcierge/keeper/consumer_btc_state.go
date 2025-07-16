package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/runtime"
)

// GetConsumerBTCState gets the unified BTC state for a specific consumer
func (k Keeper) GetConsumerBTCState(ctx context.Context, consumerID string) *types.ConsumerBTCState {
	store := k.consumerBTCStateStore(ctx)
	stateBytes := store.Get([]byte(consumerID))
	if len(stateBytes) == 0 {
		return nil
	}
	var state types.ConsumerBTCState
	k.cdc.MustUnmarshal(stateBytes, &state)
	return &state
}

// SetConsumerBTCState sets the unified BTC state for a specific consumer
func (k Keeper) SetConsumerBTCState(ctx context.Context, consumerID string, state *types.ConsumerBTCState) {
	store := k.consumerBTCStateStore(ctx)
	stateBytes := k.cdc.MustMarshal(state)
	store.Set([]byte(consumerID), stateBytes)
}

// GetConsumerBaseBTCHeader gets the base BTC header for a specific consumer
func (k Keeper) GetConsumerBaseBTCHeader(ctx context.Context, consumerID string) *btclctypes.BTCHeaderInfo {
	state := k.GetConsumerBTCState(ctx, consumerID)
	if state == nil || state.BaseHeader == nil {
		return nil
	}
	return state.BaseHeader
}

// SetConsumerBaseBTCHeader sets the base BTC header for a specific consumer
func (k Keeper) SetConsumerBaseBTCHeader(ctx context.Context, consumerID string, header *btclctypes.BTCHeaderInfo) {
	state := k.GetConsumerBTCState(ctx, consumerID)
	if state == nil {
		state = &types.ConsumerBTCState{}
	}
	state.BaseHeader = header
	k.SetConsumerBTCState(ctx, consumerID, state)
}

// GetConsumerLastSentSegment gets the last sent segment for a specific consumer
func (k Keeper) GetConsumerLastSentSegment(ctx context.Context, consumerID string) *types.BTCChainSegment {
	state := k.GetConsumerBTCState(ctx, consumerID)
	if state == nil || state.LastSentSegment == nil {
		return nil
	}
	return state.LastSentSegment
}

// SetConsumerLastSentSegment sets the last sent segment for a specific consumer
func (k Keeper) SetConsumerLastSentSegment(ctx context.Context, consumerID string, segment *types.BTCChainSegment) {
	state := k.GetConsumerBTCState(ctx, consumerID)
	if state == nil {
		state = &types.ConsumerBTCState{}
	}
	state.LastSentSegment = segment
	k.SetConsumerBTCState(ctx, consumerID, state)
}

// consumerBTCStateStore stores the unified BTC state for each consumer
// prefix: ConsumerBTCStateKey
// key: consumerID
// value: ConsumerBTCState
func (k Keeper) consumerBTCStateStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.ConsumerBTCStateKey)
}

// InitializeConsumerBTCState initializes the BTC state for a consumer
// This is called when a new IBC channel is created to set the starting point for BTC header synchronization
// The base header is set to the current BTC tip
func (k Keeper) InitializeConsumerBTCState(ctx context.Context, consumerID string) error {
	// Check if consumer state already exists
	if existingState := k.GetConsumerBTCState(ctx, consumerID); existingState != nil {
		// State already exists, no need to initialize
		return nil
	}

	// Get the current tip of the BTC light client
	tipInfo := k.btclcKeeper.GetTipInfo(ctx)
	if tipInfo == nil {
		return fmt.Errorf("BTC light client tip info is nil")
	}

	// Initialize the state with the current tip as the base header
	state := &types.ConsumerBTCState{
		BaseHeader:      tipInfo,
		LastSentSegment: nil, // No headers sent yet
	}

	k.SetConsumerBTCState(ctx, consumerID, state)
	return nil
}
