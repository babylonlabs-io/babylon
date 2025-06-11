package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/babylonlabs-io/babylon/v4/app/ante"
	"github.com/babylonlabs-io/babylon/v4/app/signer"
	cmtcfg "github.com/cometbft/cometbft/config"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	evmserverconfig "github.com/cosmos/evm/server/config"
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

type BabylonMempoolConfig struct {
	MaxGasWantedPerTx string `mapstructure:"max-gas-wanted-per-tx"`
}

func defaultBabylonMempoolConfig() BabylonMempoolConfig {
	return BabylonMempoolConfig{
		MaxGasWantedPerTx: strconv.Itoa(ante.DefaultMaxGasWantedPerTx),
	}
}

type BabylonAppConfig struct {
	serverconfig.Config `mapstructure:",squash"`

	Wasm wasmtypes.NodeConfig `mapstructure:"wasm"`

	BtcConfig BtcConfig `mapstructure:"btc-config"`

	BlsConfig BlsConfig `mapstructure:"bls-config"`

	BabylonMempoolConfig BabylonMempoolConfig `mapstructure:"babylon-mempool"`

	// EVM config
	EVM     evmserverconfig.EVMConfig     `mapstructure:"evm"`
	JSONRPC evmserverconfig.JSONRPCConfig `mapstructure:"json-rpc"`
	TLS     evmserverconfig.TLSConfig     `mapstructure:"tls"`
}

func DefaultBabylonAppConfig() *BabylonAppConfig {
	baseConfig := *serverconfig.DefaultConfig()
	// Update the default Mempool.MaxTxs to be 0 to make sure the PriorityNonceMempool is used
	baseConfig.Mempool.MaxTxs = 0
	// The SDK's default minimum gas price is set to "0.002ubbn" (empty value) inside
	// app.toml, in order to avoid spamming attacks due to transactions with 0 gas price.
	baseConfig.MinGasPrices = fmt.Sprintf("%f%s", appparams.GlobalMinGasPrice, appparams.BaseCoinUnit)
	return &BabylonAppConfig{
		Config:               baseConfig,
		Wasm:                 wasmtypes.DefaultNodeConfig(),
		BtcConfig:            defaultBabylonBtcConfig(),
		BlsConfig:            defaultBabylonBlsConfig(),
		BabylonMempoolConfig: defaultBabylonMempoolConfig(),
		EVM:                  *evmserverconfig.DefaultEVMConfig(),
		JSONRPC:              *evmserverconfig.DefaultJSONRPCConfig(),
		TLS:                  *evmserverconfig.DefaultTLSConfig(),
	}
}

func DefaultBabylonTemplate() string {
	return serverconfig.DefaultConfigTemplate + evmserverconfig.DefaultEVMConfigTemplate + wasmtypes.DefaultConfigTemplate() + `
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

###############################################################################
###                      Babylon Mempool Configuration                      ###
###############################################################################

[babylon-mempool]
# This is the max allowed gas for any tx.
# This is only for local mempool purposes, and thus	is only ran on check tx.
max-gas-wanted-per-tx = "{{ .BabylonMempoolConfig.MaxGasWantedPerTx }}"
`
}
