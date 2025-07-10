package testnet

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/babylonlabs-io/babylon/v3/app/keepers"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/v3/x/btcstaking/keeper"
	btcstakingtypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

const (
	UpgradeName = "v1rc8"
)

func CreateUpgrade() upgrades.Upgrade {
	return upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(),
	}
}

// CreateUpgradeHandler upgrade handler for launch.
func CreateUpgradeHandler() upgrades.UpgradeHandlerCreator {
	return func(mm *module.Manager, cfg module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(context context.Context, _plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			ctx := sdk.UnwrapSDKContext(context)

			logger := ctx.Logger().With("upgrade", UpgradeName)
			migrations, err := mm.RunMigrations(ctx, cfg, fromVM)
			if err != nil {
				return nil, fmt.Errorf("failed to run migrations: %w", err)
			}

			logger.Info("migrating finality providers...")
			if err := MigrateFinalityProviders(ctx, keepers.BTCStakingKeeper); err != nil {
				return nil, fmt.Errorf("failed migrate finality providers: %w", err)
			}
			logger.Info("finality providers migration done!")

			return migrations, nil
		}
	}
}

// MigrateFinalityProviders populates the new CommissionInfo field
// with default values
func MigrateFinalityProviders(goCtx context.Context, k btcstakingkeeper.Keeper) error {
	var (
		defaultCommissionMaxRate       = sdkmath.LegacyMustNewDecFromStr("0.2")
		defaultCommissionMaxChangeRate = sdkmath.LegacyMustNewDecFromStr("0.01")
		ctx                            = sdk.UnwrapSDKContext(goCtx)
		pagKey                         = []byte{}
	)
	for {
		// FinalityProviders query is paginated, so we'll need to use
		// the page key to make sure to migrate all of them
		res, err := k.FinalityProviders(goCtx, &btcstakingtypes.QueryFinalityProvidersRequest{
			Pagination: &query.PageRequest{Key: pagKey},
		})
		if err != nil {
			return err
		}

		for _, fp := range res.FinalityProviders {
			err := k.UpdateFinalityProvider(goCtx, &btcstakingtypes.FinalityProvider{
				Addr:                 fp.Addr,
				Description:          fp.Description,
				Commission:           fp.Commission,
				BtcPk:                fp.BtcPk,
				Pop:                  fp.Pop,
				SlashedBabylonHeight: fp.SlashedBabylonHeight,
				SlashedBtcHeight:     fp.SlashedBtcHeight,
				Jailed:               fp.Jailed,
				HighestVotedHeight:   fp.HighestVotedHeight,
				BsnId:                fp.BsnId, // This field is not available on release/v1.x branch. Make sure to remove it when backporting
				CommissionInfo:       btcstakingtypes.NewCommissionInfoWithTime(defaultCommissionMaxRate, defaultCommissionMaxChangeRate, ctx.BlockHeader().Time),
			})
			if err != nil {
				return err
			}
		}

		// Break if there are no more pages
		if res.Pagination == nil || len(res.Pagination.NextKey) == 0 {
			break
		}

		// Set the next pagination key
		pagKey = res.Pagination.NextKey
	}

	return nil
}
