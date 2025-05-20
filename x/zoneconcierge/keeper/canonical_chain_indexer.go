package keeper

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/runtime"

	sdkerrors "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// FindClosestHeader finds the IndexedHeader that is closest to (but not after) the given height
func (k Keeper) FindClosestHeader(ctx context.Context, consumerID string, height uint64) (*types.IndexedHeader, error) {
	chainInfo, err := k.GetChainInfo(ctx, consumerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain info for chain with ID %s: %w", consumerID, err)
	}

	// if the given height is no lower than the latest header, return the latest header directly
	if chainInfo.LatestHeader.Height <= height {
		return chainInfo.LatestHeader, nil
	}

	// the requested height is lower than the latest header, trace back until finding a timestamped header
	store := k.canonicalChainStore(ctx, consumerID)
	heightBytes := sdk.Uint64ToBigEndian(height)
	iter := store.ReverseIterator(nil, heightBytes)
	defer iter.Close()
	// if there is no key within range [0, height], return error
	if !iter.Valid() {
		return nil, fmt.Errorf("chain with ID %s does not have a timestamped header before height %d", consumerID, height)
	}
	// find the header in bytes, decode and return
	headerBytes := iter.Value()
	var header types.IndexedHeader
	k.cdc.MustUnmarshal(headerBytes, &header)
	return &header, nil
}

func (k Keeper) GetHeader(ctx context.Context, consumerID string, height uint64) (*types.IndexedHeader, error) {
	store := k.canonicalChainStore(ctx, consumerID)
	heightBytes := sdk.Uint64ToBigEndian(height)
	if !store.Has(heightBytes) {
		return nil, types.ErrHeaderNotFound
	}
	headerBytes := store.Get(heightBytes)
	var header types.IndexedHeader
	k.cdc.MustUnmarshal(headerBytes, &header)
	return &header, nil
}

func (k Keeper) insertHeader(ctx context.Context, consumerID string, header *types.IndexedHeader) error {
	if header == nil {
		return sdkerrors.Wrapf(types.ErrInvalidHeader, "header is nil")
	}
	// NOTE: we can accept header without ancestor since IBC connection can be established at any height
	store := k.canonicalChainStore(ctx, consumerID)
	store.Set(sdk.Uint64ToBigEndian(header.Height), k.cdc.MustMarshal(header))
	return nil
}

// canonicalChainStore stores the canonical chain of a Consumer, formed as a list of IndexedHeader
// prefix: CanonicalChainKey || consumerID
// key: height
// value: IndexedHeader
func (k Keeper) canonicalChainStore(ctx context.Context, consumerID string) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	canonicalChainStore := prefix.NewStore(storeAdapter, types.CanonicalChainKey)
	consumerIDBytes := []byte(consumerID)
	return prefix.NewStore(canonicalChainStore, consumerIDBytes)
}
