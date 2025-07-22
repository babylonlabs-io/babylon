package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// GetFinalizedHeader gets the finalized header for a given consumer and epoch
func (k Keeper) GetFinalizedHeader(ctx context.Context, consumerID string, epochNumber uint64) (*types.IndexedHeaderWithProof, error) {
	if !k.FinalizedHeaderExists(ctx, consumerID, epochNumber) {
		return nil, types.ErrEpochHeadersNotFound
	}

	store := k.finalizedEpochHeadersStore(ctx)
	key := k.finalizedHeaderKey(epochNumber, consumerID)
	headerBytes := store.Get(key)
	var headerWithProof types.IndexedHeaderWithProof
	k.cdc.MustUnmarshal(headerBytes, &headerWithProof)
	return &headerWithProof, nil
}

// setFinalizedHeader sets the finalized header for a given consumer and epoch
func (k Keeper) setFinalizedHeader(ctx context.Context, consumerID string, epochNumber uint64, headerWithProof *types.IndexedHeaderWithProof) {
	store := k.finalizedEpochHeadersStore(ctx)
	key := k.finalizedHeaderKey(epochNumber, consumerID)
	store.Set(key, k.cdc.MustMarshal(headerWithProof))
}

// FinalizedHeaderExists checks if a finalized header exists for a given consumer and epoch
func (k Keeper) FinalizedHeaderExists(ctx context.Context, consumerID string, epochNumber uint64) bool {
	store := k.finalizedEpochHeadersStore(ctx)
	key := k.finalizedHeaderKey(epochNumber, consumerID)
	return store.Has(key)
}

// GetAllFinalizedHeaders gets all finalized headers for iteration
func (k Keeper) GetAllFinalizedHeaders(ctx context.Context) []*types.FinalizedHeaderEntry {
	store := k.finalizedEpochHeadersStore(ctx)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var entries []*types.FinalizedHeaderEntry
	for ; iterator.Valid(); iterator.Next() {
		epochNumber, consumerID := k.parseFinalizedHeaderKey(iterator.Key())
		var headerWithProof types.IndexedHeaderWithProof
		k.cdc.MustUnmarshal(iterator.Value(), &headerWithProof)
		entries = append(entries, &types.FinalizedHeaderEntry{
			EpochNumber:     epochNumber,
			ConsumerId:      consumerID,
			HeaderWithProof: &headerWithProof,
		})
	}
	return entries
}

// ClearLatestEpochHeaders clears all latest epoch headers (called after epoch ends)
func (k Keeper) ClearLatestEpochHeaders(ctx context.Context) {
	store := k.latestEpochHeadersStore(ctx)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var keys [][]byte
	for ; iterator.Valid(); iterator.Next() {
		keys = append(keys, iterator.Key())
	}

	for _, key := range keys {
		store.Delete(key)
	}
}

// recordEpochHeaders records the headers for a given epoch number from the latest epoch headers
func (k Keeper) recordEpochHeaders(ctx context.Context, epochNumber uint64) {
	// Iterate through all latest epoch headers
	store := k.latestEpochHeadersStore(ctx)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		consumerID := string(iterator.Key())
		var header types.IndexedHeader
		k.cdc.MustUnmarshal(iterator.Value(), &header)

		// Create IndexedHeaderWithProof with nil proof for now
		headerWithProof := &types.IndexedHeaderWithProof{
			Header: &header,
			Proof:  nil,
		}

		// Store the finalized header
		k.setFinalizedHeader(ctx, consumerID, epochNumber, headerWithProof)
	}

	// Clear the latest epoch headers after recording
	k.ClearLatestEpochHeaders(ctx)
}

// recordEpochHeadersProofs records the proofs for headers of a given epoch
func (k Keeper) recordEpochHeadersProofs(ctx context.Context, epochNumber uint64) {
	curEpoch := k.GetEpoch(ctx)

	// Get all finalized headers for this epoch
	store := k.finalizedEpochHeadersStore(ctx)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		keyEpochNumber, consumerID := k.parseFinalizedHeaderKey(iterator.Key())
		if keyEpochNumber != epochNumber {
			continue
		}

		var headerWithProof types.IndexedHeaderWithProof
		k.cdc.MustUnmarshal(iterator.Value(), &headerWithProof)

		// Only generate proof if the header is from the current epoch
		if headerWithProof.Header.BabylonEpoch == curEpoch.EpochNumber {
			// Generate proof that the header is committed to the epoch
			proof, err := k.ProveConsumerHeaderInEpoch(ctx, headerWithProof.Header, curEpoch)
			if err != nil {
				panic(fmt.Errorf("failed to generate proof for consumer %s: %w", consumerID, err))
			}

			headerWithProof.Proof = proof

			// Update the stored header with proof
			k.setFinalizedHeader(ctx, consumerID, epochNumber, &headerWithProof)
		}
	}
}

// GetLatestEpochHeader gets the latest header for a consumer in the current epoch
func (k Keeper) GetLatestEpochHeader(ctx context.Context, consumerID string) *types.IndexedHeader {
	store := k.latestEpochHeadersStore(ctx)
	key := []byte(consumerID)

	bz := store.Get(key)
	if bz == nil {
		return nil
	}

	var header types.IndexedHeader
	k.cdc.MustUnmarshal(bz, &header)
	return &header
}

// SetLatestEpochHeader sets the latest header for a consumer in the current epoch
func (k Keeper) SetLatestEpochHeader(ctx context.Context, consumerID string, header *types.IndexedHeader) {
	store := k.latestEpochHeadersStore(ctx)
	key := []byte(consumerID)
	bz := k.cdc.MustMarshal(header)
	store.Set(key, bz)
}

// latestEpochHeadersStore returns the KVStore for latest epoch headers
func (k Keeper) latestEpochHeadersStore(ctx context.Context) storetypes.KVStore {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.LatestEpochHeadersKey)
}

// finalizedEpochHeadersStore stores finalized headers for each consumer and epoch
// key: epochNumber || consumerID
// value: IndexedHeaderWithProof
func (k Keeper) finalizedEpochHeadersStore(ctx context.Context) storetypes.KVStore {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.FinalizedEpochHeadersKey)
}

// finalizedHeaderKey creates a key for finalized headers
func (k Keeper) finalizedHeaderKey(epochNumber uint64, consumerID string) []byte {
	epochBytes := sdk.Uint64ToBigEndian(epochNumber)
	consumerIDBytes := []byte(consumerID)
	return append(epochBytes, consumerIDBytes...)
}

// parseFinalizedHeaderKey parses a finalized header key
func (k Keeper) parseFinalizedHeaderKey(key []byte) (uint64, string) {
	epochNumber := sdk.BigEndianToUint64(key[:8])
	consumerID := string(key[8:])
	return epochNumber, consumerID
}
