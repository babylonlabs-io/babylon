package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/babylonlabs-io/babylon/app/signer"
	cmtcfg "github.com/cometbft/cometbft/config"

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

type BlsConfig struct {
	BlsKeyFile string `mapstructure:"bls-key-file"`
}

func defaultBabylonBlsConfig() BlsConfig {
	return BlsConfig{
		BlsKeyFile: filepath.Join(cmtcfg.DefaultConfigDir, signer.DefaultBlsKeyName),
	}
}

type BabylonAppConfig struct {
	serverconfig.Config `mapstructure:",squash"`

	Wasm wasmtypes.NodeConfig `mapstructure:"wasm"`

	BtcConfig BtcConfig `mapstructure:"btc-config"`

	BlsConfig BlsConfig `mapstructure:"bls-config"`
}

func DefaultBabylonAppConfig() *BabylonAppConfig {
	baseConfig := *serverconfig.DefaultConfig()
	// The SDK's default minimum gas price is set to "0.002ubbn" (empty value) inside
	// app.toml, in order to avoid spamming attacks due to transactions with 0 gas price.
	baseConfig.MinGasPrices = fmt.Sprintf("%f%s", appparams.GlobalMinGasPrice, appparams.BaseCoinUnit)
	return &BabylonAppConfig{
		Config:    baseConfig,
		Wasm:      wasmtypes.DefaultNodeConfig(),
		BtcConfig: defaultBabylonBtcConfig(),
		BlsConfig: defaultBabylonBlsConfig(),
	}
}

func DefaultBabylonTemplate() string {
	return serverconfig.DefaultConfigTemplate + wasmtypes.DefaultConfigTemplate() + `
###############################################################################
###                        BLS configuration                                ###
###############################################################################

[bls-config]
# Path to the BLS key file (if empty, defaults to $HOME/.babylond/config/bls_key.json)
bls-key-file = "{{ .BlsConfig.BlsKeyFile }}"

###############################################################################
###                      Babylon Bitcoin configuration                      ###
###############################################################################

[btc-config]

# Configures which bitcoin network should be used for checkpointing
# valid values are: [mainnet, testnet, simnet, signet, regtest]
network = "{{ .BtcConfig.Network }}"
`
}
