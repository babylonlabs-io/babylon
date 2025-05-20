package keeper

import (
	"context"

	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	for _, c := range gs.Consumers {
		if err := k.RegisterConsumer(ctx, c); err != nil {
			return err
		}
	}

	for _, fp := range gs.FinalityProviders {
		k.SetConsumerFinalityProvider(ctx, fp)
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the module's exported genesis
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	cs, err := k.consumers(ctx)
	if err != nil {
		return nil, err
	}

	fps, err := k.finalityProviders(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:            k.GetParams(ctx),
		Consumers:         cs,
		FinalityProviders: fps,
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

func (k Keeper) finalityProviders(ctx context.Context) ([]*btcstktypes.FinalityProvider, error) {
	// need to get all the stored chain IDs from the finalityProviderChainStore
	// cause we don't know beforehand what's the chainID length that is used as
	// prefix in the fp store
	consumerIds := k.consumerIdsFromFpStore(ctx)
	fps := make([]*btcstktypes.FinalityProvider, 0)

	for _, cId := range consumerIds {
		fpStore := k.finalityProviderStore(ctx, cId)
		iter := fpStore.Iterator(nil, nil)

		for ; iter.Valid(); iter.Next() {
			var fp btcstktypes.FinalityProvider
			if err := fp.Unmarshal(iter.Value()); err != nil {
				iter.Close()
				return nil, err
			}
			fps = append(fps, &fp)
		}
		iter.Close()
	}

	return fps, nil
}

func (k Keeper) consumerIdsFromFpStore(ctx context.Context) []string {
	consumerIds := make([]string, 0)
	fpChainStore := k.finalityProviderChainStore(ctx)
	iter := fpChainStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		consumerIds = append(consumerIds, string(iter.Value()))
	}

	return consumerIds
}
