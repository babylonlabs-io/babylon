package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/runtime"
)

// GetConsumerBaseBTCHeader gets the base BTC header for a specific Consumer
func (k Keeper) GetConsumerBaseBTCHeader(ctx context.Context, consumerID string) *btclctypes.BTCHeaderInfo {
	store := k.consumerBaseBTCHeaderStore(ctx)
	headerBytes := store.Get([]byte(consumerID))
	if len(headerBytes) == 0 {
		return nil
	}
	var header btclctypes.BTCHeaderInfo
	k.cdc.MustUnmarshal(headerBytes, &header)
	return &header
}

// SetConsumerBaseBTCHeader sets the base BTC header for a specific Consumer
func (k Keeper) SetConsumerBaseBTCHeader(ctx context.Context, consumerID string, header *btclctypes.BTCHeaderInfo) {
	store := k.consumerBaseBTCHeaderStore(ctx)
	headerBytes := k.cdc.MustMarshal(header)
	store.Set([]byte(consumerID), headerBytes)
}

// consumerBaseBTCHeaderStore stores the base BTC header for each Consumer
// prefix: ConsumerBaseBTCHeaderKey
// key: consumerID
// value: BTCHeaderInfo
func (k Keeper) consumerBaseBTCHeaderStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.ConsumerBaseBTCHeaderKey)
}

// initializeConsumerBaseBTCHeader initializes the base BTC header for a Consumer
// This is called when a new IBC channel is created to set the starting point for BTC header synchronization
// NOTE: This function is currently unused as BSNs should report their own base headers via BSNBaseBTCHeaderIBCPacket
func (k Keeper) initializeConsumerBaseBTCHeader(ctx context.Context, consumerID string) error {
	// Check if Consumer base header already exists
	if existingHeader := k.GetConsumerBaseBTCHeader(ctx, consumerID); existingHeader != nil {
		// Base header already exists, no need to initialize
		return nil
	}

	// Get the current tip of the BTC light client
	tipInfo := k.btclcKeeper.GetTipInfo(ctx)
	if tipInfo == nil {
		return fmt.Errorf("BTC light client tip info is nil")
	}

	// Set the current tip as the base header for this Consumer
	k.SetConsumerBaseBTCHeader(ctx, consumerID, tipInfo)

	return nil
}
