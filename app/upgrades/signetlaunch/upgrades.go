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
	"strings"

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
	btcstktypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          "signet-launch",
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

type dataSignedFps struct {
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

		if err := propLaunch(ctx, &keepers.BTCLightClientKeeper); err != nil {
			panic(err)
		}

		return migrations, nil
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

// LoadSignedFPsFromData returns the finality providers from the json string.
// It also verifies if the msg is correctly signed and is valid to be inserted.
func LoadSignedFPsFromData(cdc codec.Codec, txDecoder sdk.TxDecoder) ([]*btcstktypes.MsgCreateFinalityProvider, error) {
	buff := bytes.NewBufferString(SignedFPsStr)

	var d dataSignedFps
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

		tx, err := txDecoder(txBytes)
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
	msgsV2, err := tx.GetMsgsV2()
	if err != nil {
		return nil, err
	}

	msgs := tx.GetMsgs()
	if len(msgs) == 0 {
		return nil, fmt.Errorf("invalid msg, there is no msg inside the tx %+v", tx)
	}

	// each tx should only contain one msg
	if len(msgs) != 1 {
		return nil, fmt.Errorf("each tx should contain only one message, invalid tx %+v", tx)
	}

	msg, ok := msgs[0].(*btcstktypes.MsgCreateFinalityProvider)
	if !ok {
		return nil, fmt.Errorf("unable to parse %+v to MsgCreateFinalityProvider", msg)
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("error validating basic msg: %w", err)
	}

	msgV2 := msgsV2[0]
	signers, err := cdc.GetMsgV2Signers(msgV2)
	if err != nil {
		return nil, fmt.Errorf("failed to get signers from msg %+v: %w", msg, err)
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("no signer at msg %+v", msgV2)
	}

	signerAddrStr, err := cdc.InterfaceRegistry().SigningContext().AddressCodec().BytesToString(signers[0])
	if err != nil {
		return nil, err
	}

	signerBbnAddr, err := sdk.AccAddressFromBech32(signerAddrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid signer address %s, err: %w", signerAddrStr, err)
	}

	if !strings.EqualFold(msg.Addr, signerAddrStr) {
		return nil, fmt.Errorf("signer address: %s is different from finality provider address: %s", signerAddrStr, msg.Addr)
	}

	if err := msg.Pop.VerifyBIP340(signerBbnAddr, msg.BtcPk); err != nil {
		return nil, fmt.Errorf("invalid Proof of Possession with signer %s: %w", signerBbnAddr.String(), err)
	}

	return msg, nil
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
