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
	var consumers []*types.ConsumerRegister
	err := k.ConsumerRegistry.Walk(ctx, nil, func(consumerID string, consumer types.ConsumerRegister) (bool, error) {
		consumers = append(consumers, &consumer)
		return false, nil
	})
	return consumers, err
}
