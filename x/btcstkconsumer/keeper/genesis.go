package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	for _, c := range gs.Consumers {
		if err := k.RegisterConsumer(ctx, c); err != nil {
			return err
		}
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the module's exported genesis
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	cs, err := k.consumers(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:    k.GetParams(ctx),
		Consumers: cs,
	}, nil
}

func (k Keeper) consumers(ctx context.Context) ([]*types.ConsumerRegister, error) {
	consumers := make([]*types.ConsumerRegister, 0)
	iter := k.consumerRegistryStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var consumer types.ConsumerRegister
		if err := consumer.Unmarshal(iter.Value()); err != nil {
			return nil, err
		}
		consumers = append(consumers, &consumer)
	}
	return consumers, nil
}
