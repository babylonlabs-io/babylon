package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

// GetEpochChainInfo gets the latest chain info of a given epoch for a given chain ID
func (k Keeper) GetEpochChainInfo(ctx context.Context, consumerID string, epochNumber uint64) (*types.ChainInfoWithProof, error) {
	if !k.EpochChainInfoExists(ctx, consumerID, epochNumber) {
		return nil, types.ErrEpochChainInfoNotFound
	}

	store := k.epochChainInfoStore(ctx, consumerID)
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	epochChainInfoBytes := store.Get(epochNumberBytes)
	var chainInfo types.ChainInfoWithProof
	k.cdc.MustUnmarshal(epochChainInfoBytes, &chainInfo)
	return &chainInfo, nil
}

func (k Keeper) setEpochChainInfo(ctx context.Context, consumerID string, epochNumber uint64, chainInfo *types.ChainInfoWithProof) {
	store := k.epochChainInfoStore(ctx, consumerID)
	store.Set(sdk.Uint64ToBigEndian(epochNumber), k.cdc.MustMarshal(chainInfo))
}

// EpochChainInfoExists checks if the latest chain info exists of a given epoch for a given chain ID
func (k Keeper) EpochChainInfoExists(ctx context.Context, consumerID string, epochNumber uint64) bool {
	store := k.epochChainInfoStore(ctx, consumerID)
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	return store.Has(epochNumberBytes)
}

// GetEpochHeaders gets the headers timestamped in a given epoch, in the ascending order
func (k Keeper) GetEpochHeaders(ctx context.Context, consumerID string, epochNumber uint64) ([]*types.IndexedHeader, error) {
	headers := []*types.IndexedHeader{}

	// find the last timestamped header of this chain in the epoch
	epochChainInfoWithProof, err := k.GetEpochChainInfo(ctx, consumerID, epochNumber)
	if err != nil {
		return nil, err
	}
	epochChainInfo := epochChainInfoWithProof.ChainInfo
	// it's possible that this epoch's snapshot is not updated for many epochs
	// this implies that this epoch does not timestamp any header for this chain at all
	if epochChainInfo.LatestHeader.BabylonEpoch < epochNumber {
		return nil, types.ErrEpochHeadersNotFound
	}
	// now we have the last header in this epoch
	headers = append(headers, epochChainInfo.LatestHeader)

	// append all previous headers until reaching the previous epoch
	canonicalChainStore := k.canonicalChainStore(ctx, consumerID)
	lastHeaderKey := sdk.Uint64ToBigEndian(epochChainInfo.LatestHeader.Height)
	// NOTE: even in ReverseIterator, start and end should still be specified in ascending order
	canonicalChainIter := canonicalChainStore.ReverseIterator(nil, lastHeaderKey)
	defer canonicalChainIter.Close()
	for ; canonicalChainIter.Valid(); canonicalChainIter.Next() {
		var prevHeader types.IndexedHeader
		k.cdc.MustUnmarshal(canonicalChainIter.Value(), &prevHeader)
		if prevHeader.BabylonEpoch < epochNumber {
			// we have reached the previous epoch, break the loop
			break
		}
		headers = append(headers, &prevHeader)
	}

	// reverse the list so that it remains ascending order
	bbn.Reverse(headers)

	return headers, nil
}

// recordEpochChainInfo records the chain info for a given epoch number of given chain ID
// where the latest chain info is retrieved from the chain info indexer
func (k Keeper) recordEpochChainInfo(ctx context.Context, consumerID string, epochNumber uint64) {
	// get the latest known chain info
	chainInfo, err := k.GetChainInfo(ctx, consumerID)
	if err != nil {
		k.Logger(sdk.UnwrapSDKContext(ctx)).Debug("chain info does not exist yet, nothing to record")
		return
	}
	chainInfoWithProof := &types.ChainInfoWithProof{
		ChainInfo:          chainInfo,
		ProofHeaderInEpoch: nil,
	}

	// NOTE: we can record epoch chain info without ancestor since IBC connection can be established at any height
	k.setEpochChainInfo(ctx, consumerID, epochNumber, chainInfoWithProof)
}

// recordEpochChainInfo records the chain info for a given epoch number of given chain ID
// where the latest chain info is retrieved from the chain info indexer
func (k Keeper) recordEpochChainInfoProofs(ctx context.Context, epochNumber uint64) {
	curEpoch := k.GetEpoch(ctx)
	consumerIDs := k.GetAllConsumerIDs(ctx)

	// save all inclusion proofs
	for _, consumerID := range consumerIDs {
		// retrieve chain info with empty proof
		chainInfo, err := k.GetEpochChainInfo(ctx, consumerID, epochNumber)
		if err != nil {
			panic(err) // only programming error
		}

		lastHeaderInEpoch := chainInfo.ChainInfo.LatestHeader
		if lastHeaderInEpoch.BabylonEpoch == curEpoch.EpochNumber {
			// get proofConsumerHeaderInEpoch
			proofConsumerHeaderInEpoch, err := k.ProveConsumerHeaderInEpoch(ctx, lastHeaderInEpoch, curEpoch)
			if err != nil {
				// only programming error is possible here
				panic(fmt.Errorf("failed to generate proofConsumerHeaderInEpoch for consumer %s: %w", consumerID, err))
			}

			chainInfo.ProofHeaderInEpoch = proofConsumerHeaderInEpoch

			// set chain info with proof back
			k.setEpochChainInfo(ctx, consumerID, epochNumber, chainInfo)
		}
	}
}

// epochChainInfoStore stores each epoch's latest ChainInfo for a Consumer
// prefix: EpochChainInfoKey || consumerID
// key: epochNumber
// value: ChainInfoWithProof
func (k Keeper) epochChainInfoStore(ctx context.Context, consumerID string) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	epochChainInfoStore := prefix.NewStore(storeAdapter, types.EpochChainInfoKey)
	consumerIDBytes := []byte(consumerID)
	return prefix.NewStore(epochChainInfoStore, consumerIDBytes)
}
