// This code is only for testing purposes.
// DO NOT USE IN PRODUCTION!

package signetlaunch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/app/upgrades"
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

		if err := propLaunch(ctx, &keepers.BTCLightClientKeeper); err != nil {
			panic(err)
		}

		return mm.RunMigrations(ctx, cfg, fromVM)
	}
}

// propLaunch runs the proposal of launch that is meant to insert new BTC Headers.
func propLaunch(
	ctx sdk.Context,
	btcLigthK *btclightkeeper.Keeper,
) error {
	newHeaders, err := LoadBTCHeadersFromData()
	if err != nil {
		return err
	}

	return insertBtcHeaders(ctx, btcLigthK, newHeaders)
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
	headers []*btclighttypes.BTCHeaderInfo,
) error {
	if len(headers) == 0 {
		return errors.New("no headers to insert")
	}

	// sort by height to make sure it is deterministic
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Height > headers[j].Height
	})

	for _, header := range headers {
		if err := header.Validate(); err != nil {
			return err
		}
	}
	k.InsertHeaderInfos(ctx, headers)

	allBlocks := k.GetMainChainFromWithLimit(ctx, 0, 1)
	isRetarget := btclighttypes.IsRetargetBlock(allBlocks[0], &chaincfg.SigNetParams)
	if !isRetarget {
		return fmt.Errorf("first header be a difficulty adjustment block")
	}
	return nil
}