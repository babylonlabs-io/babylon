package keeper

import (
	"context"
	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/store/prefix"
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

func (k Keeper) SetChainRegister(ctx context.Context, chainRegister *types.ChainRegister) {
	store := k.chainRegistryStore(ctx)
	store.Set([]byte(chainRegister.ChainId), k.cdc.MustMarshal(chainRegister))
}

// IsChainRegistered returns whether the chain register exists for a given ID
func (k Keeper) IsChainRegistered(ctx context.Context, chainID string) bool {
	store := k.chainRegistryStore(ctx)
	return store.Has([]byte(chainID))
}

// GetChainRegister returns the ChainRegister struct for a chain with a given ID.
func (k Keeper) GetChainRegister(ctx context.Context, chainID string) (*types.ChainRegister, error) {
	if !k.IsChainRegistered(ctx, chainID) {
		return nil, types.ErrChainNotRegistered
	}

	store := k.chainRegistryStore(ctx)
	chainRegisterBytes := store.Get([]byte(chainID))
	var chainRegister types.ChainRegister
	k.cdc.MustUnmarshal(chainRegisterBytes, &chainRegister)
	return &chainRegister, nil
}

// GetAllRegisteredChainIDs gets all chain IDs that registered to Babylon
func (k Keeper) GetAllRegisteredChainIDs(ctx context.Context) []string {
	chainIDs := []string{}
	iter := k.chainRegistryStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		chainIDBytes := iter.Key()
		chainID := string(chainIDBytes)
		chainIDs = append(chainIDs, chainID)
	}
	return chainIDs
}

// chainRegistryStore stores the information of registered CZ chains
// prefix: ChainRegisterKey
// key: chainID
// value: ChainRegister
func (k Keeper) chainRegistryStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.ChainRegisterKey)
}
