package keeper

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/collections"
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

/*
	Public randomness commitment storage
*/

// GetPubRandCommitForHeight finds the public randomness commitment that includes the given
// height for the given finality provider. 
// Uses binary search on the indexed start heights to find the commitment.
func (k Keeper) GetPubRandCommitForHeight(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, height uint64) (*types.PubRandCommit, error) {
	index, err := k.pubRandCommitIndex.Get(ctx, fpBtcPK.MustMarshal())
	if err != nil {
		return nil, err
	}
	heights := index.Heights
	heightsCount := len(heights)
	if heightsCount == 0 {
		return nil, types.ErrPubRandNotFound.Wrap("empty index")
	}

	// Binary search for the largest startHeight <= height
	// This Search func finds and returns the smallest index i in [0, n) at which f(i) is true,
	i := sort.Search(heightsCount, func(i int) bool {
		return heights[i] > height
	})
	if i == 0 {
		// height is less than the smallest start height
		return nil, types.ErrPubRandNotFound.Wrap("height is less than the smallest start height in index")
	}
	// we want startHeight <= height
	// Thus, we're interested in the heights[i-1]
	startHeight := heights[i-1]

	// Fetch the commitment from the store
	store := k.pubRandCommitFpStore(ctx, fpBtcPK)
	bz := store.Get(sdk.Uint64ToBigEndian(startHeight))
	if bz == nil {
		return nil, types.ErrPubRandNotFound.Wrapf("startHeight: %d", startHeight)
	}

	var prCommit types.PubRandCommit
	k.cdc.MustUnmarshal(bz, &prCommit)
	if !prCommit.IsInRange(height) {
		return nil, types.ErrPubRandNotFound.Wrapf("height %d is not in range of found PubRandCommit with startHeight %d and NumPubRand %d", height, startHeight, prCommit.NumPubRand)
	}

	return &prCommit, nil
}


// GetTimestampedPubRandCommitForHeight finds the public randomness commitment that includes the given
// height for the given finality provider
func (k Keeper) GetTimestampedPubRandCommitForHeight(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, height uint64) (*types.PubRandCommit, error) {
	prCommit, err := k.GetPubRandCommitForHeight(ctx, fpBtcPK, height)
	if err != nil {
		return nil, err
	}

	// ensure the finality provider's last randomness commit is already finalised by BTC timestamping
	finalizedEpoch := k.GetLastFinalizedEpoch(ctx)
	if finalizedEpoch == 0 {
		return nil, types.ErrPubRandCommitNotBTCTimestamped.Wrapf("no finalized epoch yet")
	}
	if finalizedEpoch < prCommit.EpochNum {
		return nil, types.ErrPubRandCommitNotBTCTimestamped.
			Wrapf("the finality provider %s last committed epoch number: %d, last finalized epoch number: %d",
				fpBtcPK.MarshalHex(), prCommit.EpochNum, finalizedEpoch)
	}

	return prCommit, nil
}

func (k Keeper) HasTimestampedPubRand(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, height uint64) bool {
	_, err := k.GetTimestampedPubRandCommitForHeight(ctx, fpBtcPK, height)
	return err == nil
}

// SetPubRandCommit adds the given public randomness commitment for the given public key
func (k Keeper) SetPubRandCommit(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, prCommit *types.PubRandCommit) error {
	store := k.pubRandCommitFpStore(ctx, fpBtcPK)
	prcBytes := k.cdc.MustMarshal(prCommit)
	store.Set(sdk.Uint64ToBigEndian(prCommit.StartHeight), prcBytes)
	// update or create the pub random commit index for the FP
	return k.upsertPubRandCommitIdx(ctx, fpBtcPK, prCommit.StartHeight)
}

// GetLastPubRandCommit retrieves the last public randomness commitment of the given finality provider
func (k Keeper) GetLastPubRandCommit(ctx context.Context, fpBtcPK *bbn.BIP340PubKey) *types.PubRandCommit {
	store := k.pubRandCommitFpStore(ctx, fpBtcPK)
	iter := store.ReverseIterator(nil, nil)
	defer iter.Close()

	if !iter.Valid() {
		// this finality provider does not commit any randomness
		return nil
	}

	var prCommit types.PubRandCommit
	k.cdc.MustUnmarshal(iter.Value(), &prCommit)
	return &prCommit
}

// pubRandCommitFpStore returns the KVStore of the commitment of public randomness
// prefix: PubRandKey
// key: (finality provider PK || block height of the commitment)
// value: PubRandCommit
func (k Keeper) pubRandCommitFpStore(ctx context.Context, fpBtcPK *bbn.BIP340PubKey) prefix.Store {
	store := k.pubRandCommitStore(ctx)
	return prefix.NewStore(store, fpBtcPK.MustMarshal())
}

// pubRandCommitStore returns the KVStore of the public randomness commitments
// prefix: PubRandKey
// key: (prefix)
// value: PubRandCommit
func (k Keeper) pubRandCommitStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.PubRandCommitKey)
}

/*
	Public randomness storage
	TODO: remove public randomness storage?
*/

// SetPubRand sets a public randomness at a given height for a given finality provider
func (k Keeper) SetPubRand(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, height uint64, pubRand bbn.SchnorrPubRand) {
	store := k.pubRandFpStore(ctx, fpBtcPK)
	store.Set(sdk.Uint64ToBigEndian(height), pubRand)
}

func (k Keeper) HasPubRand(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, height uint64) bool {
	store := k.pubRandFpStore(ctx, fpBtcPK)
	return store.Has(sdk.Uint64ToBigEndian(height))
}

func (k Keeper) GetPubRand(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, height uint64) (*bbn.SchnorrPubRand, error) {
	store := k.pubRandFpStore(ctx, fpBtcPK)
	prBytes := store.Get(sdk.Uint64ToBigEndian(height))
	if len(prBytes) == 0 {
		return nil, types.ErrPubRandNotFound
	}
	return bbn.NewSchnorrPubRand(prBytes)
}

// GetLastPubRand retrieves the last public randomness committed by the given finality provider
func (k Keeper) GetLastPubRand(ctx context.Context, fpBtcPK *bbn.BIP340PubKey) (uint64, *bbn.SchnorrPubRand, error) {
	store := k.pubRandFpStore(ctx, fpBtcPK)
	iter := store.ReverseIterator(nil, nil)
	defer iter.Close()

	if !iter.Valid() {
		// this finality provider does not commit any randomness
		return 0, nil, types.ErrNoPubRandYet
	}

	height := sdk.BigEndianToUint64(iter.Key())
	pubRand, err := bbn.NewSchnorrPubRand(iter.Value())
	if err != nil {
		// failing to marshal public randomness in KVStore can only be a programming error
		panic(fmt.Errorf("failed to unmarshal public randomness in KVStore: %w", err))
	}
	return height, pubRand, nil
}

// pubRandFpStore returns the KVStore of the public randomness
// prefix: PubRandKey
// key: (finality provider PK || block height)
// value: PublicRandomness
func (k Keeper) pubRandFpStore(ctx context.Context, fpBtcPK *bbn.BIP340PubKey) prefix.Store {
	prefixedStore := k.pubRandStore(ctx)
	return prefix.NewStore(prefixedStore, fpBtcPK.MustMarshal())
}

// pubRandStore returns the KVStore of the public randomness
// prefix: PubRandKey
// key: (prefix)
// value: PublicRandomness
func (k Keeper) pubRandStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.PubRandKey)
}

// upsertPubRandCommitIdx appends a new startHeight to the pubRandCommitIndex for the given
// finality provider public key, ensuring that the heights remain strictly increasing.
// If the index does not exist yet, it is created.
// Returns an error if the new startHeight is not greater than the last recorded height.
func (k Keeper) upsertPubRandCommitIdx(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, startHeight uint64) error {
	key := fpBtcPK.MustMarshal()
	index, err := k.pubRandCommitIndex.Get(ctx, key)
	if err != nil {
		// If no existing index, initialize a new one
		if errors.Is(err, collections.ErrNotFound) {
			index = types.PubRandCommitIndexValue{}
		} else {
			return err
		}
	}

	// Ensure the new startHeight is strictly greater than the last one
	n := len(index.Heights)
	if n > 0 && startHeight <= index.Heights[n-1] {
		return fmt.Errorf("startHeight %d must be greater than the last committed height %d", startHeight, index.Heights[n-1])
	}

	// Append the new height to the index
	index.Heights = append(index.Heights, startHeight)

	// Persist the updated index
	return k.pubRandCommitIndex.Set(ctx, key, index)
}
