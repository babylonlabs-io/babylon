package v3rc3

import (
	"context"
	"errors"

<<<<<<< HEAD
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
=======
	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/types/module"
>>>>>>> 3bd5721 (fix: `LargestBtcReorg` prefix to follow mainnet (#1608))
)

// UpgradeName defines the on-chain upgrade name for the Babylon v3rc3 upgrade
const UpgradeName = "v3rc3"

var (
	// Babylon Mainnet has the LargestBtcReorgInBlocks at prefix 11, at PR https://github.com/babylonlabs-io/babylon/pull/518
	// it was modified to prefix 13 and only applied to testnet. This will be changed in this upgrade so that all branches
	// and babylon chains will follow mainnet and have LargestBtcReorgInBlocks at prefix 11. Luckily the prefix 11 in testnet
	// was being reserved by BTCCOnsumerDelegatorKey which was not being used and it can be revoked and cleaned.
	OldTestnetLargestBtcReorgInBlocks = collections.NewPrefix(13)

	// Upgrade for version v3rc3
	Upgrade = upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler,
		StoreUpgrades: store.StoreUpgrades{
			Added:   []string{},
			Deleted: []string{},
		},
	}
)

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// Run migrations before applying any other state changes.
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		err = upgrades.FpSoftDeleteDupAddr(ctx, keepers.BTCStakingKeeper)
		if err != nil {
			return nil, err
		}

		btcStkStoreKey := keepers.GetKey(btcstktypes.StoreKey)
		if btcStkStoreKey == nil {
			return nil, errors.New("invalid btcstaking types store key")
		}

		btcStkStoreService := runtime.NewKVStoreService(btcStkStoreKey)
		err = UpdatePrefixLargestBtcReorgInBlocks(ctx, keepers.EncCfg.Codec, btcStkStoreService, keepers.BTCStakingKeeper)
		if err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

func UpdatePrefixLargestBtcReorgInBlocks(
	ctx context.Context,
	cdc codec.BinaryCodec,
	btcStkStoreService corestoretypes.KVStoreService,
	btcStkK btcstakingkeeper.Keeper,
) error {
	sb := collections.NewSchemaBuilder(btcStkStoreService)
	oldLargestBtcReorgItem := collections.NewItem(
		sb,
		OldTestnetLargestBtcReorgInBlocks,
		"largest_btc_reorg",
		codec.CollValue[btcstktypes.LargestBtcReOrg](cdc),
	)

	largestBtcReorg, err := btcStkK.LargestBtcReorg.Get(ctx)
	oldLargestBtcReorg, errOld := oldLargestBtcReorgItem.Get(ctx)

	btcReorgToSet := GetLargestBtcReorg(largestBtcReorg, oldLargestBtcReorg, err, errOld)
	if btcReorgToSet == nil { // nothing to set, just clean...
		if err := btcStkK.LargestBtcReorg.Remove(ctx); err != nil {
			return err
		}
	} else {
		// there is something to set
		if err := btcStkK.SetLargestBtcReorg(ctx, *btcReorgToSet); err != nil {
			return err
		}
	}

	// last thing is to clean the prefix 13
	return oldLargestBtcReorgItem.Remove(ctx)
}

func GetLargestBtcReorg(largestBtcReorg, oldLargestBtcReorg btcstktypes.LargestBtcReOrg, err, errOld error) *btcstktypes.LargestBtcReOrg {
	if err != nil && errOld != nil {
		// no valid largest btc reorg, should clean it all
		return nil
	}

	// only the prefix 13 has a valid largest
	if err != nil {
		return &oldLargestBtcReorg
	}

	// only the prefix 11 has a valid largest
	if errOld != nil {
		return &largestBtcReorg
	}

	// both prefixes have valid largest btc reorgs, get the largest diff than
	if oldLargestBtcReorg.BlockDiff > largestBtcReorg.BlockDiff {
		return &oldLargestBtcReorg
	}
	return &largestBtcReorg
}
