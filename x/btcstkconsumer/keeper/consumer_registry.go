package keeper

import (
	"context"

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

// GetAllRegisteredConsumerIDs gets all consumer IDs that registered to Babylon
func (k Keeper) GetAllRegisteredConsumerIDs(ctx context.Context) []string {
	var consumerIDs []string
	err := k.ConsumerRegistry.Walk(ctx, nil, func(consumerID string, _ types.ConsumerRegister) (bool, error) {
		consumerIDs = append(consumerIDs, consumerID)
		return false, nil
	})
	if err != nil {
		panic(err)
	}
	return consumerIDs
}
