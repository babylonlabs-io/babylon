package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

// RegisterConsumer registers a new consumer
func (k Keeper) RegisterConsumer(ctx context.Context, consumerRegister *types.ConsumerRegister) error {
	if k.IsConsumerRegistered(ctx, consumerRegister.ConsumerId) {
		return types.ErrConsumerAlreadyRegistered
	}
	return k.setConsumerRegister(ctx, consumerRegister)
}

// UpdateConsumer updates the consumer register for a given consumer ID
func (k Keeper) UpdateConsumer(ctx context.Context, consumerRegister *types.ConsumerRegister) error {
	if !k.IsConsumerRegistered(ctx, consumerRegister.ConsumerId) {
		return types.ErrConsumerNotRegistered
	}
	return k.setConsumerRegister(ctx, consumerRegister)
}

func (k Keeper) setConsumerRegister(ctx context.Context, consumerRegister *types.ConsumerRegister) error {
	return k.ConsumerRegistry.Set(ctx, consumerRegister.ConsumerId, *consumerRegister)
}

// IsConsumerRegistered returns whether the consumer register exists for a given ID
func (k Keeper) IsConsumerRegistered(ctx context.Context, consumerID string) bool {
	has, err := k.ConsumerRegistry.Has(ctx, consumerID)
	if err != nil {
		return false
	}
	return has
}

// GetConsumerRegister returns the ConsumerRegister struct for a consumer with a given ID.
func (k Keeper) GetConsumerRegister(ctx context.Context, consumerID string) (*types.ConsumerRegister, error) {
	consumerRegister, err := k.ConsumerRegistry.Get(ctx, consumerID)
	if err != nil {
		return nil, types.ErrConsumerNotRegistered
	}
	return &consumerRegister, nil
}

func (k Keeper) IsCosmosConsumer(ctx context.Context, consumerID string) (bool, error) {
	consumerRegister, err := k.GetConsumerRegister(ctx, consumerID)
	if err != nil {
		return false, err
	}
	return consumerRegister.GetCosmosConsumerMetadata() != nil, nil
}

// GetAllRegisteredCosmosConsumers gets all cosmos consumers that registered to Babylon
func (k Keeper) GetAllRegisteredCosmosConsumers(ctx context.Context) []*types.ConsumerRegister {
	var consumers []*types.ConsumerRegister
	err := k.ConsumerRegistry.Walk(ctx, nil, func(consumerID string, consumerRegister types.ConsumerRegister) (bool, error) {
		if consumerRegister.GetCosmosConsumerMetadata() != nil {
			consumers = append(consumers, &consumerRegister)
		}
		return false, nil
	})
	if err != nil {
		panic(err)
	}
	return consumers
}

// GetConsumerID returns the consumer ID based on the channel and port ID
func (k Keeper) GetConsumerID(ctx sdk.Context, portID, channelID string) (consumerID string, err error) {
	clientID, _, err := k.channelKeeper.GetChannelClientState(ctx, portID, channelID)
	if err != nil {
		return "", channeltypes.ErrChannelNotFound.Wrapf("portID: %s, channelID: %s - %v", portID, channelID, err)
	}

	cons, err := k.GetConsumerRegister(ctx, clientID)
	if err != nil {
		return "", err
	}

	return cons.ConsumerId, nil
}

func (k Keeper) ConsumerHasIBCChannelOpen(ctx context.Context, consumerID, channelID string) bool {
	return k.channelKeeper.ConsumerHasIBCChannelOpen(ctx, consumerID, channelID)
}
