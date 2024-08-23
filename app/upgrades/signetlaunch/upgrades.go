// This code is only for testing purposes.
// DO NOT USE IN PRODUCTION!

package signetlaunch

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	bbn "github.com/babylonlabs-io/babylon/types"
	btclightkeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          "signet-launch",
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

// CreateUpgradeHandler upgrade handler for launch.
func CreateUpgradeHandler(
	mm *module.Manager,
	cfg module.Configurator,
	app upgrades.BaseAppParamManager,
	keepers *keepers.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(context context.Context, _plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(context)

		migrations, err := mm.RunMigrations(ctx, cfg, fromVM)
		if err != nil {
			return nil, err
		}

		if err := propLaunch(ctx, &keepers.BTCLightClientKeeper); err != nil {
			panic(err)
		}

		return migrations, nil
	}
}

// propLaunch runs the proposal of launch that is meant to insert new BTC Headers.
func propLaunch(
	ctx sdk.Context,
	btcLightK *btclightkeeper.Keeper,
) error {
	newHeaders, err := LoadBTCHeadersFromData()
	if err != nil {
		return err
	}

	return insertBtcHeaders(ctx, btcLightK, newHeaders)
}

// LoadBTCHeadersFromData returns the BTC headers load from the json string with the headers inside of it.
func LoadBTCHeadersFromData() ([]*btclighttypes.BTCHeaderInfo, error) {
	cdc := appparams.DefaultEncodingConfig().Codec
	buff := bytes.NewBufferString(NewBtcHeadersStr)

	var gs btclighttypes.GenesisState
	err := cdc.UnmarshalJSON(buff.Bytes(), &gs)
	if err != nil {
		return nil, err
	}

	return gs.BtcHeaders, nil
}

func insertBtcHeaders(
	ctx sdk.Context,
	k *btclightkeeper.Keeper,
	btcHeaders []*btclighttypes.BTCHeaderInfo,
) error {
	if len(btcHeaders) == 0 {
		return errors.New("no headers to insert")
	}

	headersBytes := make([]bbn.BTCHeaderBytes, len(btcHeaders))
	for i, btcHeader := range btcHeaders {
		h := btcHeader
		headersBytes[i] = *h.Header
	}

	if err := k.InsertHeaders(ctx, headersBytes); err != nil {
		return err
	}

	allBlocks := k.GetMainChainFromWithLimit(ctx, 0, 1)
	isRetarget := btclighttypes.IsRetargetBlock(allBlocks[0], &chaincfg.SigNetParams)
	if !isRetarget {
		return fmt.Errorf("first header be a difficulty adjustment block")
	}
	return nil
}
