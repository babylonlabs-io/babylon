// This code is only for testing purposes.
// DO NOT USE IN PRODUCTION!

package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"sort"

	sdkmath "cosmossdk.io/math"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	accountkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/babylonlabs-io/babylon/v3/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclightkeeper "github.com/babylonlabs-io/babylon/v3/x/btclightclient/keeper"
	btclighttypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/v3/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	finalitykeeper "github.com/babylonlabs-io/babylon/v3/x/finality/keeper"
	finalitytypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
	incentivekeeper "github.com/babylonlabs-io/babylon/v3/x/incentive/keeper"
	incentivetypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	mintkeeper "github.com/babylonlabs-io/babylon/v3/x/mint/keeper"
	minttypes "github.com/babylonlabs-io/babylon/v3/x/mint/types"
)

type ParamUpgradeFn func(ctx sdk.Context, k *keepers.AppKeepers) error

const (
	ZoneConciergeStoreKey = "zoneconcierge"
	UpgradeName           = "v1"
)

func CreateUpgrade(upgradeDataStr UpgradeDataString, parmUpgradeFn ParamUpgradeFn) upgrades.Upgrade {
	return upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(upgradeDataStr, parmUpgradeFn),
		// Upgrade necessary for deletions of `zoneconcierge`
		StoreUpgrades: store.StoreUpgrades{
			Deleted: []string{ZoneConciergeStoreKey},
		},
	}
}

// CreateUpgradeHandler upgrade handler for launch.
func CreateUpgradeHandler(upgradeDataStr UpgradeDataString, parmUpgradeFn ParamUpgradeFn) upgrades.UpgradeHandlerCreator {
	return func(mm *module.Manager, cfg module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(context context.Context, _plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			ctx := sdk.UnwrapSDKContext(context)

			migrations, err := mm.RunMigrations(ctx, cfg, fromVM)
			if err != nil {
				return nil, fmt.Errorf("failed to run migrations: %w", err)
			}

			// Re-initialise the mint module as we have replaced Cosmos SDK's
			// mint module with our own one.
			err = upgradeMint(
				ctx,
				&keepers.MintKeeper,
				&keepers.AccountKeeper,
				keepers.StakingKeeper,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to upgrade mint module: %w", err)
			}

			err = upgradeParameters(
				ctx,
				keepers.EncCfg.Codec,
				&keepers.BTCStakingKeeper,
				&keepers.FinalityKeeper,
				&keepers.IncentiveKeeper,
				&keepers.WasmKeeper,
				upgradeDataStr.BtcStakingParamsStr,
				upgradeDataStr.FinalityParamStr,
				upgradeDataStr.IncentiveParamStr,
				upgradeDataStr.CosmWasmParamStr,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to upgrade parameters: %w", err)
			}

			err = upgradeLaunch(
				ctx,
				keepers.EncCfg,
				&keepers.BTCLightClientKeeper,
				&keepers.BTCStakingKeeper,
				keepers.BankKeeper,
				upgradeDataStr.NewBtcHeadersStr,
				upgradeDataStr.TokensDistributionStr,
				upgradeDataStr.AllowedStakingTxHashesStr,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to upgrade inserting additional data: %w", err)
			}

			// if there is hardcoded upgrade for parameters, run it
			if parmUpgradeFn != nil {
				err = parmUpgradeFn(ctx, keepers)
				if err != nil {
					return nil, fmt.Errorf("failed to upgrade parameters: %w", err)
				}
			}

			return migrations, nil
		}
	}
}

func upgradeMint(
	ctx sdk.Context,
	k *mintkeeper.Keeper,
	ak *accountkeeper.AccountKeeper,
	stk *stakingkeeper.Keeper,
) error {
	bondedDenom, err := stk.BondDenom(ctx)
	if err != nil {
		return err
	}
	minter := minttypes.DefaultMinter()
	minter.BondDenom = bondedDenom
	k.InitGenesis(ctx, ak, &minttypes.GenesisState{
		Minter: &minter,
	})
	return nil
}

func upgradeParameters(
	ctx sdk.Context,
	cdc codec.Codec,
	btcK *btcstkkeeper.Keeper,
	finK *finalitykeeper.Keeper,
	iK *incentivekeeper.Keeper,
	wasmK *wasmkeeper.Keeper,
	btcStakingParam, finalityParam, incentiveParam, wasmParam string,
) error {
	// Upgrade the staking parameters as first, as other upgrades depend on it.
	if err := upgradeBtcStakingParameters(ctx, btcK, btcStakingParam); err != nil {
		return fmt.Errorf("failed to upgrade btc staking parameters: %w", err)
	}
	if err := upgradeFinalityParameters(ctx, cdc, finK, finalityParam); err != nil {
		return fmt.Errorf("failed to upgrade finality parameters: %w", err)
	}
	if err := upgradeIncentiveParameters(ctx, cdc, iK, incentiveParam); err != nil {
		return fmt.Errorf("failed to upgrade incentive parameters: %w", err)
	}

	if err := upgradeCosmWasmParameters(ctx, cdc, wasmK, wasmParam); err != nil {
		return fmt.Errorf("failed to upgrade cosmwasm parameters: %w", err)
	}

	return nil
}

func upgradeIncentiveParameters(
	ctx sdk.Context,
	cdc codec.Codec,
	k *incentivekeeper.Keeper,
	incentiveParam string,
) error {
	params, err := LoadIncentiveParamsFromData(cdc, incentiveParam)
	if err != nil {
		return err
	}

	return k.SetParams(ctx, params)
}

func upgradeCosmWasmParameters(
	ctx sdk.Context,
	cdc codec.Codec,
	k *wasmkeeper.Keeper,
	wasmParam string,
) error {
	params, err := LoadCosmWasmParamsFromData(cdc, wasmParam)
	if err != nil {
		return err
	}

	return k.SetParams(ctx, params)
}

func upgradeBtcStakingParameters(
	ctx sdk.Context,
	k *btcstkkeeper.Keeper,
	btcStakingParam string,
) error {
	// params should be already sorted by their btc activation
	// block height in ascending order
	params, err := LoadBtcStakingParamsFromData(btcStakingParam)
	if err != nil {
		return err
	}

	for version, p := range params {
		if err := k.OverwriteParamsAtVersion(ctx, uint32(version), p); err != nil {
			return err
		}
	}
	return nil
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
func upgradeLaunch(
	ctx sdk.Context,
	encCfg sdktestutil.TestEncodingConfig,
	btcLigthK *btclightkeeper.Keeper,
	btcK *btcstkkeeper.Keeper,
	bankK bankkeeper.SendKeeper,
	btcHeaders, tokensDistribution, allowedStakingTxHashes string,
) error {
	if err := upgradeTokensDistribution(ctx, bankK, tokensDistribution); err != nil {
		return fmt.Errorf("failed to upgrade tokens distribution: %w", err)
	}

	if err := upgradeAllowedStakingTransactions(ctx, btcK, allowedStakingTxHashes); err != nil {
		return fmt.Errorf("failed to upgrade allowed staking transactions: %w", err)
	}

	if err := upgradeBTCHeaders(ctx, encCfg.Codec, btcLigthK, btcHeaders); err != nil {
		return fmt.Errorf("failed to upgrade btc headers: %w", err)
	}

	return nil
}

func upgradeTokensDistribution(ctx sdk.Context, bankK bankkeeper.SendKeeper, tokensDistribution string) error {
	data, err := LoadTokenDistributionFromData(tokensDistribution)
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

func upgradeAllowedStakingTransactions(ctx sdk.Context, btcStakingK *btcstkkeeper.Keeper, allowedStakingTxHashes string) error {
	data, err := LoadAllowedStakingTransactionHashesFromData(allowedStakingTxHashes)
	if err != nil {
		return fmt.Errorf("failed to load allowed staking transaction hashes from string %s: %w", allowedStakingTxHashes, err)
	}

	for _, txHash := range data.TxHashes {
		hash, err := chainhash.NewHashFromStr(txHash)
		if err != nil {
			return fmt.Errorf("failed to parse tx hash: %w", err)
		}
		btcStakingK.IndexAllowedStakingTransaction(ctx, hash)
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

func LoadBtcStakingParamsFromData(data string) ([]btcstktypes.Params, error) {
	buff := bytes.NewBufferString(data)

	var params []btcstktypes.Params
	err := json.Unmarshal(buff.Bytes(), &params)
	if err != nil {
		return []btcstktypes.Params{}, err
	}

	// sort params by the BTC activation height ascending order 100, 150, 200...
	sort.Slice(params, func(i, j int) bool {
		return params[i].BtcActivationHeight < params[j].BtcActivationHeight
	})

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

func LoadIncentiveParamsFromData(cdc codec.Codec, data string) (incentivetypes.Params, error) {
	buff := bytes.NewBufferString(data)

	var params incentivetypes.Params
	err := cdc.UnmarshalJSON(buff.Bytes(), &params)
	if err != nil {
		return incentivetypes.Params{}, err
	}

	return params, nil
}

func LoadCosmWasmParamsFromData(cdc codec.Codec, data string) (wasmtypes.Params, error) {
	buff := bytes.NewBufferString(data)

	var params wasmtypes.Params
	err := cdc.UnmarshalJSON(buff.Bytes(), &params)
	if err != nil {
		return wasmtypes.Params{}, err
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

// LoadTokenDistributionFromData returns the tokens to be distributed from the json string.
func LoadTokenDistributionFromData(data string) (DataTokenDistribution, error) {
	buff := bytes.NewBufferString(data)

	var d DataTokenDistribution
	err := json.Unmarshal(buff.Bytes(), &d)
	if err != nil {
		return d, err
	}

	return d, nil
}

func LoadAllowedStakingTransactionHashesFromData(data string) (*AllowedStakingTransactionHashes, error) {
	buff := bytes.NewBufferString(data)

	var d AllowedStakingTransactionHashes
	err := json.Unmarshal(buff.Bytes(), &d)
	if err != nil {
		return nil, err
	}

	return &d, nil
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
