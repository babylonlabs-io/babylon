package cmd

import (
	"fmt"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

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

func DefaultBabylonAppConfig() *BabylonAppConfig {
	baseConfig := *serverconfig.DefaultConfig()
	// The SDK's default minimum gas price is set to "0.002ubbn" (empty value) inside
	// app.toml, in order to avoid spamming attacks due to transactions with 0 gas price.
	baseConfig.MinGasPrices = fmt.Sprintf("%f%s", appparams.GlobalMinGasPrice, appparams.BaseCoinUnit)
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
