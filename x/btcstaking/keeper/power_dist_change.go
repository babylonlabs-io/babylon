package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

/* voting power distribution update event store */

// addPowerDistUpdateEvent appends an event that affect voting power distribution
// to the store
func (k Keeper) addPowerDistUpdateEvent(
	ctx context.Context,
	btcHeight uint32,
	event *types.EventPowerDistUpdate,
) {
	store := k.PowerDistUpdateEventBtcHeightStore(ctx, btcHeight)

	// get event index
	eventIdx := uint64(0) // event index starts from 0
	iter := store.ReverseIterator(nil, nil)
	defer iter.Close()
	if iter.Valid() {
		// if there exists events already, event index will be the subsequent one
		eventIdx = sdk.BigEndianToUint64(iter.Key()) + 1
	}

	// key is event index, and value is the event bytes
	store.Set(sdk.Uint64ToBigEndian(eventIdx), k.cdc.MustMarshal(event))
}

// ClearPowerDistUpdateEvents removes all BTC delegation state update events
// at a given BTC height
// This is called after processing all BTC delegation events in `BeginBlocker`
// nolint:unused
func (k Keeper) ClearPowerDistUpdateEvents(ctx context.Context, btcHeight uint32) {
	store := k.PowerDistUpdateEventBtcHeightStore(ctx, btcHeight)
	keys := [][]byte{}

	// get all keys
	// using an enclosure to ensure iterator is closed right after
	// the function is done
	func() {
		iter := store.Iterator(nil, nil)
		defer iter.Close()
		for ; iter.Valid(); iter.Next() {
			keys = append(keys, iter.Key())
		}
	}()

	// remove all keys
	for _, key := range keys {
		store.Delete(key)
	}
}

// GetAllPowerDistUpdateEvents gets all voting power update events
func (k Keeper) GetAllPowerDistUpdateEvents(ctx context.Context, lastBTCTip uint32, curBTCTip uint32) []*types.EventPowerDistUpdate {
	events := []*types.EventPowerDistUpdate{}
	for i := lastBTCTip; i <= curBTCTip; i++ {
		k.iteratePowerDistUpdateEvents(ctx, i, func(event *types.EventPowerDistUpdate) bool {
			events = append(events, event)
			return true
		})
	}
	return events
}

// iteratePowerDistUpdateEvents uses the given handler function to handle each
// voting power distribution update event that happens at the given BTC height.
// This is called in `BeginBlocker`
func (k Keeper) iteratePowerDistUpdateEvents(
	ctx context.Context,
	btcHeight uint32,
	handleFunc func(event *types.EventPowerDistUpdate) bool,
) {
	store := k.PowerDistUpdateEventBtcHeightStore(ctx, btcHeight)
	iter := store.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var event types.EventPowerDistUpdate
		k.cdc.MustUnmarshal(iter.Value(), &event)
		shouldContinue := handleFunc(&event)
		if !shouldContinue {
			break
		}
	}
}

// PowerDistUpdateEventBtcHeightStore returns the KVStore of events that affect
// voting power distribution
// prefix: PowerDistUpdateKey || BTC height
// key: event index)
// value: BTCDelegationStatus
func (k Keeper) PowerDistUpdateEventBtcHeightStore(ctx context.Context, btcHeight uint32) prefix.Store {
	store := k.powerDistUpdateEventStore(ctx)
	return prefix.NewStore(store, sdk.Uint64ToBigEndian(uint64(btcHeight)))
}

// powerDistUpdateEventStore returns the KVStore of events that affect
// voting power distribution
// prefix: PowerDistUpdateKey
// key: (BTC height || event index)
// value: BTCDelegationStatus
func (k Keeper) powerDistUpdateEventStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.PowerDistUpdateKey)
}
