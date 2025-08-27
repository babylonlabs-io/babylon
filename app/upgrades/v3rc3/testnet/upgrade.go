package testnet

import (
	"context"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/btcstaking"
	bsckeeper "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/keeper"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		if err = btcstaking.FpSoftDeleteDupAddr(ctx, keepers.BTCStakingKeeper); err != nil {
			return nil, err
		}

		if err := IndexFinalityContracts(ctx, keepers.BTCStkConsumerKeeper); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

// IndexFinalityContracts indexes all finality contracts for registered rollup consumers
// NOTE: this is only needed for testnet
func IndexFinalityContracts(ctx context.Context, k bsckeeper.Keeper) error {
	return k.ConsumerRegistry.Walk(ctx, nil, func(consumerID string, consumerRegister bsctypes.ConsumerRegister) (bool, error) {
		// Select consumers registered as Cosmos consumer with metadata, IBC init complete with channel ID set
		metadata := consumerRegister.GetRollupConsumerMetadata()
		if metadata == nil || metadata.FinalityContractAddress == "" {
			return false, nil
		}
		return false, k.RegisterFinalityContract(ctx, metadata.FinalityContractAddress)
	})
}
