package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/store/prefix"
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

func (k Keeper) SetConsumerRegister(ctx context.Context, consumerRegister *types.ConsumerRegister) {
	store := k.consumerRegistryStore(ctx)
	store.Set([]byte(consumerRegister.ConsumerId), k.cdc.MustMarshal(consumerRegister))
}

// IsConsumerRegistered returns whether the consumer register exists for a given ID
func (k Keeper) IsConsumerRegistered(ctx context.Context, consumerID string) bool {
	store := k.consumerRegistryStore(ctx)
	return store.Has([]byte(consumerID))
}

// GetConsumerRegister returns the ConsumerRegister struct for a consumer with a given ID.
func (k Keeper) GetConsumerRegister(ctx context.Context, consumerID string) (*types.ConsumerRegister, error) {
	if !k.IsConsumerRegistered(ctx, consumerID) {
		return nil, types.ErrConsumerNotRegistered
	}

	store := k.consumerRegistryStore(ctx)
	consumerRegisterBytes := store.Get([]byte(consumerID))
	var consumerRegister types.ConsumerRegister
	k.cdc.MustUnmarshal(consumerRegisterBytes, &consumerRegister)
	return &consumerRegister, nil
}

// GetAllRegisteredConsumerIDs gets all consumer IDs that registered to Babylon
func (k Keeper) GetAllRegisteredConsumerIDs(ctx context.Context) []string {
	consumerIDs := []string{}
	iter := k.consumerRegistryStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		consumerIDBytes := iter.Key()
		consumerID := string(consumerIDBytes)
		consumerIDs = append(consumerIDs, consumerID)
	}
	return consumerIDs
}

// consumerRegistryStore stores the information of registered CZ consumers
// prefix: ConsumerRegisterKey
// key: consumerID
// value: ConsumerRegister
func (k Keeper) consumerRegistryStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.ConsumerRegisterKey)
}
