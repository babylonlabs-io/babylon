package keeper

import (
	"context"
	errorsmod "cosmossdk.io/errors"
	"github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/store/prefix"
	bbn "github.com/babylonchain/babylon/types"
	btcstaking "github.com/babylonchain/babylon/x/btcstaking/types"
	btcstktypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

// SetFinalityProvider adds the given finality provider to CZ chains KVStore
func (k Keeper) SetFinalityProvider(ctx context.Context, fp *btcstaking.FinalityProvider) {
	store := k.finalityProviderStore(ctx, fp.ChainId)
	fpBytes := k.cdc.MustMarshal(fp)
	store.Set(fp.BtcPk.MustMarshal(), fpBytes)
}

// GetFinalityProvider gets the finality provider in the chain id with the given finality provider Bitcoin PK
func (k Keeper) GetFinalityProvider(ctx context.Context, chainID string, fpBTCPK *types.BIP340PubKey) (*btcstaking.FinalityProvider, error) {
	if !k.HasFinalityProvider(ctx, chainID, fpBTCPK) {
		return nil, btcstaking.ErrFpNotFound
	}
	store := k.finalityProviderStore(ctx, chainID)
	fpBytes := store.Get(*fpBTCPK)
	var fp btcstaking.FinalityProvider
	k.cdc.MustUnmarshal(fpBytes, &fp)
	return &fp, nil
}

// HasFinalityProvider checks whether the finality provider exists and is registered for a given (registered) chain ID
func (k Keeper) HasFinalityProvider(ctx context.Context, chainID string, fpBTCPK *types.BIP340PubKey) bool {
	store := k.finalityProviderStore(ctx, chainID)
	return store.Has(*fpBTCPK)
}

// IterateFPs iterates over all finality providers for a given chain
func (k Keeper) IterateFPs(ctx context.Context, chainId string, handler func(fp *btcstaking.FinalityProvider) bool) {
	// filter out all finality providers with positive voting power
	fpIter := k.finalityProviderStore(ctx, chainId).Iterator(nil, nil)
	defer fpIter.Close()
	for ; fpIter.Valid(); fpIter.Next() {
		var fp btcstaking.FinalityProvider
		k.cdc.MustUnmarshal(fpIter.Value(), &fp)

		shouldContinue := handler(&fp)
		if !shouldContinue {
			return
		}
	}
}

// HasFPs checks if the FPs list exists for a given chain id
func (k Keeper) HasFPs(ctx context.Context, chainID string) bool {
	store := k.finalityProviderStore(ctx, chainID)
	iter := store.Iterator(nil, nil)
	defer iter.Close()
	return iter.Valid()
}

// GetFPs gets the list of FPs, i.e., the finality provider set for a given chain id
func (k Keeper) GetFPs(ctx context.Context, chainID string) map[string]btcstaking.FinalityProvider {
	store := k.finalityProviderStore(ctx, chainID)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	// if no finality provider for this chain id, return nil
	if !iter.Valid() {
		return nil
	}

	// get all finality providers for this chain id
	fpSet := map[string]btcstaking.FinalityProvider{}
	for ; iter.Valid(); iter.Next() {
		fpBTCPK, err := bbn.NewBIP340PubKey(iter.Key())
		if err != nil {
			// failing to unmarshal finality provider BTC PK in KVStore is a programming error
			panic(errorsmod.Wrapf(bbn.ErrUnmarshal, "Failed to unmarshall FP BTC PK %s: %v", iter.Key(), err))
		}
		var fp btcstaking.FinalityProvider
		k.cdc.MustUnmarshal(iter.Value(), &fp)
		fpSet[fpBTCPK.MarshalHex()] = fp
	}

	return fpSet
}

// finalityProviderStore returns the KVStore of the finality provider set per chain
// prefix: FinalityProviderKey || chain id
// key: Bitcoin PubKey
// value: FinalityProvider
func (k Keeper) finalityProviderStore(ctx context.Context, chainID string) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	fpStore := prefix.NewStore(storeAdapter, btcstktypes.FinalityProviderKey)
	return prefix.NewStore(fpStore, []byte(chainID))
}
