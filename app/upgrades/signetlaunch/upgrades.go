// This code is only for testing purposes.
// DO NOT USE IN PRODUCTION!

package signetlaunch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	bbn "github.com/babylonlabs-io/babylon/types"
	btclightkeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          "signet-launch",
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

type DataSignedFps struct {
	SignedTxsFP []any `json:"signed_txs_create_fp"`
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

		// Upgrade the staking parameters as first, as other upgrades depend on it.
		if err := upgradeBtcStakingParameters(ctx, keepers.EncCfg, &keepers.BTCStakingKeeper); err != nil {
			panic(err)
		}

		if err := propLaunch(ctx, keepers.EncCfg, &keepers.BTCLightClientKeeper, &keepers.BTCStakingKeeper); err != nil {
			panic(err)
		}

		return migrations, nil
	}
}

func LoadBtcStakingParamsFromData(cdc codec.Codec) (btcstktypes.Params, error) {
	buff := bytes.NewBufferString(BtcStakingParamStr)

	var params btcstktypes.Params
	err := cdc.UnmarshalJSON(buff.Bytes(), &params)
	if err != nil {
		return btcstktypes.Params{}, err
	}

	return params, nil
}

func upgradeBtcStakingParameters(
	ctx sdk.Context,
	e *appparams.EncodingConfig,
	k *btcstkkeeper.Keeper,
) error {

	cdc := e.Codec

	params, err := LoadBtcStakingParamsFromData(cdc)

	if err != nil {
		return err
	}

	// We are overwriting the params at version 0, as the upgrade is happening from
	// TGE chain so there should be only one version of the params
	return k.OverwriteParamsAtVersion(ctx, 0, params)
}

// propLaunch runs the proposal of launch that is meant to insert new BTC Headers.
func propLaunch(
	ctx sdk.Context,
	encCfg *appparams.EncodingConfig,
	btcLigthK *btclightkeeper.Keeper,
	btcStkK *btcstkkeeper.Keeper,
) error {
	cdc := encCfg.Codec

	newHeaders, err := LoadBTCHeadersFromData(cdc)
	if err != nil {
		return err
	}

	if err := insertBtcHeaders(ctx, btcLigthK, newHeaders); err != nil {
		return err
	}

	fps, err := LoadSignedFPsFromData(cdc, encCfg.TxConfig.TxJSONDecoder())
	if err != nil {
		return err
	}

	return insertFPs(ctx, btcStkK, fps)
}

// LoadBTCHeadersFromData returns the BTC headers load from the json string with the headers inside of it.
func LoadBTCHeadersFromData(cdc codec.Codec) ([]*btclighttypes.BTCHeaderInfo, error) {
	buff := bytes.NewBufferString(NewBtcHeadersStr)

	var gs btclighttypes.GenesisState
	err := cdc.UnmarshalJSON(buff.Bytes(), &gs)
	if err != nil {
		return nil, err
	}

	return gs.BtcHeaders, nil
}

// LoadSignedFPsFromData returns the finality providers from the json string.
func LoadSignedFPsFromData(cdc codec.Codec, txJSONDecoder sdk.TxDecoder) ([]*btcstktypes.MsgCreateFinalityProvider, error) {
	buff := bytes.NewBufferString(SignedFPsStr)

	var d DataSignedFps
	err := json.Unmarshal(buff.Bytes(), &d)
	if err != nil {
		return nil, err
	}

	fps := make([]*btcstktypes.MsgCreateFinalityProvider, len(d.SignedTxsFP))
	for i, txAny := range d.SignedTxsFP {
		txBytes, err := json.Marshal(txAny)
		if err != nil {
			return nil, err
		}

		tx, err := txJSONDecoder(txBytes)
		if err != nil {
			return nil, err
		}

		fp, err := parseCreateFPFromSignedTx(cdc, tx)
		if err != nil {
			return nil, err
		}

		fps[i] = fp
	}

	// sorts all the FPs by their addresses
	sort.Slice(fps, func(i, j int) bool {
		return fps[i].Addr > fps[j].Addr
	})

	return fps, nil
}

func parseCreateFPFromSignedTx(cdc codec.Codec, tx sdk.Tx) (*btcstktypes.MsgCreateFinalityProvider, error) {
	msgs := tx.GetMsgs()
	if len(msgs) != 1 {
		return nil, fmt.Errorf("each tx should contain only one message, invalid tx %+v", tx)
	}

	msg, ok := msgs[0].(*btcstktypes.MsgCreateFinalityProvider)
	if !ok {
		return nil, fmt.Errorf("unable to parse %+v to MsgCreateFinalityProvider", msg)
	}

	return msg, nil
}

func insertFPs(
	ctx sdk.Context,
	k *btcstkkeeper.Keeper,
	fps []*btcstktypes.MsgCreateFinalityProvider,
) error {
	for _, fp := range fps {
		if err := k.AddFinalityProvider(ctx, fp); err != nil {
			return err
		}
	}

	return nil
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
