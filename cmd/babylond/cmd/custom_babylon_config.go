package cmd

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	sdkmath "cosmossdk.io/math"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	bbn "github.com/babylonlabs-io/babylon/types"
)

type BtcConfig struct {
	Network string `mapstructure:"network"`
}

func defaultBabylonBtcConfig() BtcConfig {
	return BtcConfig{
		Network: string(bbn.BtcMainnet),
	}
}

type BabylonAppConfig struct {
	serverconfig.Config `mapstructure:",squash"`

	Wasm wasmtypes.WasmConfig `mapstructure:"wasm"`

	BtcConfig BtcConfig `mapstructure:"btc-config"`
}

func DefaultBabylonConfig() *BabylonAppConfig {
	baseConfig := *serverconfig.DefaultConfig()
	baseConfig.MinGasPrices = sdk.NewCoin(appparams.BaseCoinUnit, sdkmath.NewInt(0)).String()
	return &BabylonAppConfig{
		Config:    baseConfig,
		Wasm:      wasmtypes.DefaultWasmConfig(),
		BtcConfig: defaultBabylonBtcConfig(),
	}
}

func DefaultBabylonTemplate() string {
	return serverconfig.DefaultConfigTemplate + wasmtypes.DefaultConfigTemplate() + `
###############################################################################
###                      Babylon Bitcoin configuration                      ###
###############################################################################

[btc-config]

# Configures which bitcoin network should be used for checkpointing
# valid values are: [mainnet, testnet, simnet, signet, regtest]
network = "{{ .BtcConfig.Network }}"
`
}
