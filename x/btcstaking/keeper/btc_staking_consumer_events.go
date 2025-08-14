package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/runtime"
)

// AddBTCStakingConsumerEvent adds event that must be sent to cosmos consumer zones
// if the provided consumer is not a cosmos consumer this function will be noop
// Consumer must be already registered in btcstkconsumer module, if not, function will panic.
func (k Keeper) AddBTCStakingConsumerEvent(ctx context.Context, consumerID string, event *types.BTCStakingConsumerEvent) error {
	return k.AddBTCStakingConsumerEvents(ctx, consumerID, []*types.BTCStakingConsumerEvent{event})
}

// AddBTCStakingConsumerEvents adds events that must be sent to cosmos consumer zones
// if the provided consumer is not a cosmos consumer this function will be noop
// Consumer must be already registered in btcstkconsumer module, if not, function will panic.
func (k Keeper) AddBTCStakingConsumerEvents(ctx context.Context, consumerID string, events []*types.BTCStakingConsumerEvent) error {
	cosmosConsumer, err := k.BscKeeper.IsCosmosConsumer(ctx, consumerID)

	if err != nil {
		// panic if consumer is not registered in btcstkconsumer module
		panic(fmt.Errorf("failed to check if consumer %s is a cosmos consumer: %w", consumerID, err))
	}

	if !cosmosConsumer {
		return nil
	}

	store := k.btcStakingConsumerEventStore(ctx)
	storeKey := []byte(consumerID)

	var packet types.BTCStakingIBCPacket
	if store.Has(storeKey) {
		eventsBytes := store.Get(storeKey)
		k.cdc.MustUnmarshal(eventsBytes, &packet)
	}

	for _, event := range events {
		switch {
		case event.GetNewFp() != nil:
			packet.NewFp = append(packet.NewFp, event.GetNewFp())
		case event.GetActiveDel() != nil:
			packet.ActiveDel = append(packet.ActiveDel, event.GetActiveDel())
		case event.GetUnbondedDel() != nil:
			packet.UnbondedDel = append(packet.UnbondedDel, event.GetUnbondedDel())
		default:
			return fmt.Errorf("unrecognized event type for event %+v", event)
		}
	}

	eventsBytes := k.cdc.MustMarshal(&packet)

	if len(eventsBytes) > types.MaxBtcStakingPacketSize {
		return fmt.Errorf("IBC packet size is too large %d bytes. Cannot create more staking events in the block", len(eventsBytes))
	}

	store.Set(storeKey, eventsBytes)

	return nil
}

// GetBTCStakingConsumerIBCPacket gets BTC staking consumer IBC packet for a given consumer ID.
func (k Keeper) GetBTCStakingConsumerIBCPacket(ctx context.Context, consumerID string) *types.BTCStakingIBCPacket {
	store := k.btcStakingConsumerEventStore(ctx)
	storeKey := []byte(consumerID)
	if !store.Has(storeKey) {
		return nil
	}

	var events types.BTCStakingIBCPacket
	eventsBytes := store.Get(storeKey)
	k.cdc.MustUnmarshal(eventsBytes, &events)
	return &events
}

// GetAllBTCStakingConsumerIBCPackets retrieves all BTC staking consumer IBC packets from the store,
// returning a map where the keys are consumer IDs and the values are the corresponding BTCStakingIBCPacket.
func (k Keeper) GetAllBTCStakingConsumerIBCPackets(ctx context.Context) map[string]*types.BTCStakingIBCPacket {
	store := k.btcStakingConsumerEventStore(ctx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	// Initialize the map to hold consumer ID keys and IBC packet values.
	allEvents := make(map[string]*types.BTCStakingIBCPacket)
	for ; iter.Valid(); iter.Next() {
		var events types.BTCStakingIBCPacket
		k.cdc.MustUnmarshal(iter.Value(), &events)
		consumerID := string(iter.Key())
		allEvents[consumerID] = &events
	}

	return allEvents
}

func (k Keeper) DeleteBTCStakingConsumerIBCPacket(ctx context.Context, consumerID string) {
	store := k.btcStakingConsumerEventStore(ctx)
	storeKey := []byte(consumerID)
	store.Delete(storeKey)
}

// btcStakingConsumerEventStore returns the KVStore of the btc staking event
// prefix: BTCStakingEventKey || consumer ID
// key: BTCStakingConsumerEventType
// value: BTCStakingConsumerEvents (a list of BTCStakingConsumerEvent)
func (k Keeper) btcStakingConsumerEventStore(ctx context.Context) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdaptor, types.BTCStakingEventKey)
}
