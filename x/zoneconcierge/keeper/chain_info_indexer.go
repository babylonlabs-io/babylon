package keeper

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/runtime"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

func (k Keeper) setChainInfo(ctx context.Context, chainInfo *types.ChainInfo) {
	store := k.chainInfoStore(ctx)
	store.Set([]byte(chainInfo.ConsumerId), k.cdc.MustMarshal(chainInfo))
}

func (k Keeper) InitChainInfo(ctx context.Context, consumerID string) (*types.ChainInfo, error) {
	if len(consumerID) == 0 {
		return nil, fmt.Errorf("consumerID is empty")
	}
	// ensure chain info has not been initialised yet
	if k.HasChainInfo(ctx, consumerID) {
		return nil, errorsmod.Wrapf(types.ErrInvalidChainInfo, "chain info has already initialized")
	}

	chainInfo := &types.ChainInfo{
		ConsumerId:              consumerID,
		LatestHeader:            nil,
		TimestampedHeadersCount: 0,
	}

	k.setChainInfo(ctx, chainInfo)
	return chainInfo, nil
}

// HasChainInfo returns whether the chain info exists for a given ID
// Since IBC does not provide API that allows to initialise chain info right before creating an IBC connection,
// we can only check its existence every time, and return an empty one if it's not initialised yet.
func (k Keeper) HasChainInfo(ctx context.Context, consumerId string) bool {
	store := k.chainInfoStore(ctx)
	return store.Has([]byte(consumerId))
}

// GetChainInfo returns the ChainInfo struct for a chain with a given ID
// Since IBC does not provide API that allows to initialise chain info right before creating an IBC connection,
// we can only check its existence every time, and return an empty one if it's not initialised yet.
func (k Keeper) GetChainInfo(ctx context.Context, consumerId string) (*types.ChainInfo, error) {
	if !k.HasChainInfo(ctx, consumerId) {
		return nil, types.ErrChainInfoNotFound
	}

	store := k.chainInfoStore(ctx)
	chainInfoBytes := store.Get([]byte(consumerId))
	var chainInfo types.ChainInfo
	k.cdc.MustUnmarshal(chainInfoBytes, &chainInfo)
	return &chainInfo, nil
}

// updateLatestHeader updates the chainInfo w.r.t. the given header, including
// - replace the old latest header with the given one
// - increment the number of timestamped headers
// Note that this function is triggered only upon receiving headers from the relayer,
// and only a subset of headers in the Consumer are relayed. Thus TimestampedHeadersCount is not
// equal to the total number of headers in the Consumer.
func (k Keeper) updateLatestHeader(ctx context.Context, consumerId string, header *types.IndexedHeader) error {
	if header == nil {
		return errorsmod.Wrapf(types.ErrInvalidHeader, "header is nil")
	}
	chainInfo, err := k.GetChainInfo(ctx, consumerId)
	if err != nil {
		// chain info has not been initialised yet
		return fmt.Errorf("failed to get chain info of %s: %w", consumerId, err)
	}
	chainInfo.LatestHeader = header     // replace the old latest header with the given one
	chainInfo.TimestampedHeadersCount++ // increment the number of timestamped headers

	k.setChainInfo(ctx, chainInfo)
	return nil
}

// GetAllConsumerIDs gets IDs of all consumer that integrate Babylon
func (k Keeper) GetAllConsumerIDs(ctx context.Context) []string {
	consumerIds := []string{}
	iter := k.chainInfoStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		consumerIdBytes := iter.Key()
		consumerId := string(consumerIdBytes)
		consumerIds = append(consumerIds, consumerId)
	}
	return consumerIds
}

// msgChainInfoStore stores the information of canonical chains and forks for Consumers
// prefix: ChainInfoKey
// key: consumerId
// value: ChainInfo
func (k Keeper) chainInfoStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.ChainInfoKey)
}
