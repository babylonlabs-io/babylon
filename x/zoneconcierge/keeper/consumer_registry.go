package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// AddConsumer adds a consumer to the registry
func (k Keeper) AddConsumer(ctx context.Context, consumerID string) {
	store := k.registeredConsumersStore(ctx)
	key := []byte(consumerID)
	store.Set(key, []byte{1}) // Just existence marker
}

// RemoveConsumer removes a consumer from the registry
func (k Keeper) RemoveConsumer(ctx context.Context, consumerID string) {
	store := k.registeredConsumersStore(ctx)
	key := []byte(consumerID)
	store.Delete(key)
}

// HasConsumer checks if a consumer is registered
func (k Keeper) HasConsumer(ctx context.Context, consumerID string) bool {
	store := k.registeredConsumersStore(ctx)
	key := []byte(consumerID)
	return store.Has(key)
}

// GetAllConsumerIDs returns all registered consumer IDs
func (k Keeper) GetAllConsumerIDs(ctx context.Context) []string {
	store := k.registeredConsumersStore(ctx)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var consumerIDs []string
	for ; iterator.Valid(); iterator.Next() {
		consumerID := string(iterator.Key())
		consumerIDs = append(consumerIDs, consumerID)
	}

	return consumerIDs
}

// registeredConsumersStore returns the KVStore for registered consumers
func (k Keeper) registeredConsumersStore(ctx context.Context) storetypes.KVStore {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.RegisteredConsumersKey)
}
