package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

// GetFinalizedHeader gets the finalized header for a given consumer and epoch
func (k Keeper) GetFinalizedHeader(ctx context.Context, consumerID string, epochNumber uint64) (*types.IndexedHeaderWithProof, error) {
	if !k.FinalizedHeaderExists(ctx, consumerID, epochNumber) {
		return nil, types.ErrEpochHeadersNotFound
	}

	key := collections.Join(epochNumber, consumerID)
	headerWithProof, err := k.FinalizedEpochHeaders.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return &headerWithProof, nil
}

// setFinalizedHeader sets the finalized header for a given consumer and epoch
func (k Keeper) setFinalizedHeader(ctx context.Context, consumerID string, epochNumber uint64, headerWithProof *types.IndexedHeaderWithProof) {
	key := collections.Join(epochNumber, consumerID)
	if err := k.FinalizedEpochHeaders.Set(ctx, key, *headerWithProof); err != nil {
		panic(err)
	}
}

// FinalizedHeaderExists checks if a finalized header exists for a given consumer and epoch
func (k Keeper) FinalizedHeaderExists(ctx context.Context, consumerID string, epochNumber uint64) bool {
	key := collections.Join(epochNumber, consumerID)
	has, err := k.FinalizedEpochHeaders.Has(ctx, key)
	if err != nil {
		return false
	}
	return has
}

// GetAllFinalizedHeaders gets all finalized headers for iteration
func (k Keeper) GetAllFinalizedHeaders(ctx context.Context) []*types.FinalizedHeaderEntry {
	var entries []*types.FinalizedHeaderEntry
	err := k.FinalizedEpochHeaders.Walk(ctx, nil, func(key collections.Pair[uint64, string], value types.IndexedHeaderWithProof) (bool, error) {
		entries = append(entries, &types.FinalizedHeaderEntry{
			EpochNumber:     key.K1(),
			ConsumerId:      key.K2(),
			HeaderWithProof: &value,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}
	return entries
}

// ClearLatestEpochHeaders clears all latest epoch headers (called after epoch ends)
func (k Keeper) ClearLatestEpochHeaders(ctx context.Context) {
	err := k.LatestEpochHeaders.Walk(ctx, nil, func(key string, value types.IndexedHeader) (bool, error) {
		return false, k.LatestEpochHeaders.Remove(ctx, key)
	})
	if err != nil {
		panic(err)
	}
}

// recordEpochHeaders records the headers for a given epoch number from the latest epoch headers
func (k Keeper) recordEpochHeaders(ctx context.Context, epochNumber uint64) {
	// Iterate through all latest epoch headers
	err := k.LatestEpochHeaders.Walk(ctx, nil, func(consumerID string, header types.IndexedHeader) (bool, error) {
		// Create IndexedHeaderWithProof with nil proof for now
		headerWithProof := &types.IndexedHeaderWithProof{
			Header: &header,
			Proof:  nil,
		}

		// Store the finalized header
		k.setFinalizedHeader(ctx, consumerID, epochNumber, headerWithProof)
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	// Clear the latest epoch headers after recording
	k.ClearLatestEpochHeaders(ctx)
}

// recordEpochHeadersProofs records the proofs for headers of a given epoch
func (k Keeper) recordEpochHeadersProofs(ctx context.Context, epochNumber uint64) {
	curEpoch := k.GetEpoch(ctx)

	// Get all finalized headers for this epoch
	err := k.FinalizedEpochHeaders.Walk(ctx, nil, func(key collections.Pair[uint64, string], headerWithProof types.IndexedHeaderWithProof) (bool, error) {
		keyEpochNumber := key.K1()
		consumerID := key.K2()

		if keyEpochNumber != epochNumber {
			return false, nil
		}

		// Only generate proof if the header is from the current epoch
		if headerWithProof.Header.BabylonEpoch == curEpoch.EpochNumber {
			// Generate proof that the header is committed to the epoch
			proof, err := k.ProveConsumerHeaderInEpoch(ctx, headerWithProof.Header, curEpoch)
			if err != nil {
				return true, fmt.Errorf("failed to generate proof for consumer %s: %w", consumerID, err)
			}

			headerWithProof.Proof = proof

			// Update the stored header with proof
			k.setFinalizedHeader(ctx, consumerID, epochNumber, &headerWithProof)
		}
		return false, nil
	})
	if err != nil {
		panic(err)
	}
}

// GetLatestEpochHeader gets the latest header for a consumer in the current epoch
func (k Keeper) GetLatestEpochHeader(ctx context.Context, consumerID string) *types.IndexedHeader {
	header, err := k.LatestEpochHeaders.Get(ctx, consumerID)
	if err != nil {
		return nil
	}
	return &header
}

// SetLatestEpochHeader sets the latest header for a consumer in the current epoch
func (k Keeper) SetLatestEpochHeader(ctx context.Context, consumerID string, header *types.IndexedHeader) {
	if err := k.LatestEpochHeaders.Set(ctx, consumerID, *header); err != nil {
		panic(err)
	}
}
