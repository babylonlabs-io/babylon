// This code is only for testing purposes.
// DO NOT USE IN PRODUCTION!

package vanilla

import (
	"context"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/app/keepers"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          "vanilla",
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func CreateUpgradeHandler(
	mm *module.Manager,
	cfg module.Configurator,
	_ upgrades.BaseAppParamManager,
	keepers *keepers.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(context context.Context, _plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(context)

		migrations, err := mm.RunMigrations(ctx, cfg, fromVM)
		if err != nil {
			return nil, err
		}

		propVanilla(ctx, &keepers.AccountKeeper, &keepers.BTCStakingKeeper)

		return migrations, nil
	}
}

func propVanilla(
	ctx sdk.Context,
	accountKeeper *authkeeper.AccountKeeper,
	bsKeeper *btcstakingkeeper.Keeper,
) {
	// remove the account with higher number and the lowest is the new fp addr
	allAccounts := accountKeeper.GetAllAccounts(ctx)
	var (
		accToRemove sdk.AccountI
		accFp       sdk.AccountI
	)
	heighestAccNumber, lowestAccNumber := uint64(0), uint64(len(allAccounts))

	for _, acc := range allAccounts {
		accNumber := acc.GetAccountNumber()
		if accNumber > heighestAccNumber {
			heighestAccNumber = accNumber
			accToRemove = acc
		}
		if accNumber < lowestAccNumber {
			lowestAccNumber = accNumber
			accFp = acc
		}
	}

	accountKeeper.RemoveAccount(ctx, accToRemove)

	// insert a FP from predefined public key
	pk, err := btcec.ParsePubKey(
		[]byte{0x06, 0x79, 0xbe, 0x66, 0x7e, 0xf9, 0xdc, 0xbb,
			0xac, 0x55, 0xa0, 0x62, 0x95, 0xce, 0x87, 0x0b, 0x07,
			0x02, 0x9b, 0xfc, 0xdb, 0x2d, 0xce, 0x28, 0xd9, 0x59,
			0xf2, 0x81, 0x5b, 0x16, 0xf8, 0x17, 0x98, 0x48, 0x3a,
			0xda, 0x77, 0x26, 0xa3, 0xc4, 0x65, 0x5d, 0xa4, 0xfb,
			0xfc, 0x0e, 0x11, 0x08, 0xa8, 0xfd, 0x17, 0xb4, 0x48,
			0xa6, 0x85, 0x54, 0x19, 0x9c, 0x47, 0xd0, 0x8f, 0xfb,
			0x10, 0xd4, 0xb8,
		},
	)
	if err != nil {
		panic(err)
	}

	btcPK := bbn.NewBIP340PubKeyFromBTCPK(pk)
	fp := &bstypes.FinalityProvider{
		Addr:  accFp.GetAddress().String(),
		BtcPk: btcPK,
	}
	bsKeeper.SetFinalityProvider(ctx, fp)
}
