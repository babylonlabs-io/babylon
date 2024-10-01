// This code is only for testing purposes.
// DO NOT USE IN PRODUCTION!

package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	sdkmath "cosmossdk.io/math"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"github.com/babylonlabs-io/babylon/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	"github.com/babylonlabs-io/babylon/app/upgrades/v1/mainnet"
	bbn "github.com/babylonlabs-io/babylon/types"
	btclightkeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	finalitykeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
	finalitytypes "github.com/babylonlabs-io/babylon/x/finality/types"
)

const (
	ZoneConciergeStoreKey = "zoneconcierge"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          "v1",
	CreateUpgradeHandler: CreateUpgradeHandler,
	// Upgrade necessary for deletions of `zoneconcierge`
	StoreUpgrades: store.StoreUpgrades{
		Deleted: []string{ZoneConciergeStoreKey},
	},
}

type UpdateStringData struct {
	BTCStakingParam   string
	FinalityParam     string
	BTCHeaders        string
	FinalityProviders string
}

type DataSignedFps struct {
	SignedTxsFP []any `json:"signed_txs_create_fp"`
}

type DataTokenDistribution struct {
	TokenDistribution []struct {
		AddressSender   string `json:"address_sender"`
		AddressReceiver string `json:"address_receiver"`
		Amount          int64  `json:"amount"`
	} `json:"token_distribution"`
}

// CreateUpgradeHandler upgrade handler for launch.
func CreateUpgradeHandler(
	mm *module.Manager,
	cfg module.Configurator,
	keepers *keepers.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(context context.Context, _plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(context)

		migrations, err := mm.RunMigrations(ctx, cfg, fromVM)
		if err != nil {
			return nil, err
		}

		if err := upgradeParameters(ctx, keepers.EncCfg.Codec, &keepers.BTCStakingKeeper, &keepers.FinalityKeeper, mainnet.BtcStakingParamStr, mainnet.FinalityParamStr); err != nil {
			return nil, err
		}

		if err := upgradeLaunch(ctx, keepers.EncCfg, &keepers.BTCLightClientKeeper, &keepers.BTCStakingKeeper, keepers.BankKeeper, mainnet.NewBtcHeadersStr, mainnet.SignedFPsStr); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

func upgradeParameters(
	ctx sdk.Context,
	cdc codec.Codec,
	btcK *btcstkkeeper.Keeper,
	finK *finalitykeeper.Keeper,
	btcStakingParam, finalityParam string,
) error {
	// Upgrade the staking parameters as first, as other upgrades depend on it.
	if err := upgradeBtcStakingParameters(ctx, cdc, btcK, btcStakingParam); err != nil {
		return err
	}

	return upgradeFinalityParameters(ctx, cdc, finK, finalityParam)
}

func upgradeBtcStakingParameters(
	ctx sdk.Context,
	cdc codec.Codec,
	k *btcstkkeeper.Keeper,
	btcStakingParam string,
) error {
	params, err := LoadBtcStakingParamsFromData(cdc, btcStakingParam)
	if err != nil {
		return err
	}

	// We are overwriting the params at version 0, as the upgrade is happening from
	// TGE chain so there should be only one version of the params
	return k.OverwriteParamsAtVersion(ctx, 0, params)
}

func upgradeFinalityParameters(
	ctx sdk.Context,
	cdc codec.Codec,
	k *finalitykeeper.Keeper,
	finalityParam string,
) error {
	params, err := LoadFinalityParamsFromData(cdc, finalityParam)
	if err != nil {
		return err
	}

	return k.SetParams(ctx, params)
}

// upgradeLaunch runs the upgrade:
// - Transfer ubbn funds for token distribution
// - Insert new BTC Headers
// - Insert new finality providers
func upgradeLaunch(
	ctx sdk.Context,
	encCfg *appparams.EncodingConfig,
	btcLigthK *btclightkeeper.Keeper,
	btcStkK *btcstkkeeper.Keeper,
	bankK bankkeeper.SendKeeper,
	btcHeaders, fps string,
) error {
	if err := upgradeTokensDistribution(ctx, bankK); err != nil {
		return err
	}

	if err := upgradeBTCHeaders(ctx, encCfg.Codec, btcLigthK, btcHeaders); err != nil {
		return err
	}

	return upgradeSignedFPs(ctx, encCfg, btcStkK, fps)
}

func upgradeTokensDistribution(ctx sdk.Context, bankK bankkeeper.SendKeeper) error {
	data, err := LoadTokenDistributionFromData()
	if err != nil {
		return err
	}

	for _, td := range data.TokenDistribution {
		receiver, err := sdk.AccAddressFromBech32(td.AddressReceiver)
		if err != nil {
			return err
		}

		sender, err := sdk.AccAddressFromBech32(td.AddressSender)
		if err != nil {
			return err
		}

		amount := sdk.NewCoin(appparams.BaseCoinUnit, sdkmath.NewInt(td.Amount))
		if err := bankK.SendCoins(ctx, sender, receiver, sdk.NewCoins(amount)); err != nil {
			return err
		}
	}

	return nil
}

func upgradeBTCHeaders(ctx sdk.Context, cdc codec.Codec, btcLigthK *btclightkeeper.Keeper, btcHeaders string) error {
	newHeaders, err := LoadBTCHeadersFromData(cdc, btcHeaders)
	if err != nil {
		return err
	}

	return insertBtcHeaders(ctx, btcLigthK, newHeaders)
}

func upgradeSignedFPs(ctx sdk.Context, encCfg *appparams.EncodingConfig, btcStkK *btcstkkeeper.Keeper, fps string) error {
	msgCreateFps, err := LoadSignedFPsFromData(encCfg.Codec, encCfg.TxConfig.TxJSONDecoder(), fps)
	if err != nil {
		return err
	}

	return insertFPs(ctx, btcStkK, msgCreateFps)
}

func LoadBtcStakingParamsFromData(cdc codec.Codec, data string) (btcstktypes.Params, error) {
	buff := bytes.NewBufferString(data)

	var params btcstktypes.Params
	err := cdc.UnmarshalJSON(buff.Bytes(), &params)
	if err != nil {
		return btcstktypes.Params{}, err
	}

	return params, nil
}

func LoadFinalityParamsFromData(cdc codec.Codec, data string) (finalitytypes.Params, error) {
	buff := bytes.NewBufferString(data)

	var params finalitytypes.Params
	err := cdc.UnmarshalJSON(buff.Bytes(), &params)
	if err != nil {
		return finalitytypes.Params{}, err
	}

	return params, nil
}

// LoadBTCHeadersFromData returns the BTC headers load from the json string with the headers inside of it.
func LoadBTCHeadersFromData(cdc codec.Codec, data string) ([]*btclighttypes.BTCHeaderInfo, error) {
	buff := bytes.NewBufferString(data)

	var gs btclighttypes.GenesisState
	err := cdc.UnmarshalJSON(buff.Bytes(), &gs)
	if err != nil {
		return nil, err
	}

	return gs.BtcHeaders, nil
}

// LoadSignedFPsFromData returns the finality providers from the json string.
func LoadSignedFPsFromData(cdc codec.Codec, txJSONDecoder sdk.TxDecoder, data string) ([]*btcstktypes.MsgCreateFinalityProvider, error) {
	buff := bytes.NewBufferString(data)

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

// LoadTokenDistributionFromData returns the tokens to be distributed from the json string.
func LoadTokenDistributionFromData() (DataTokenDistribution, error) {
	buff := bytes.NewBufferString(TokensDistribution)

	var d DataTokenDistribution
	err := json.Unmarshal(buff.Bytes(), &d)
	if err != nil {
		return d, err
	}

	return d, nil
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
