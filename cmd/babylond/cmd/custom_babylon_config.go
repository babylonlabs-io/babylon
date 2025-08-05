package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/babylonlabs-io/babylon/v3/app/ante"
	"github.com/babylonlabs-io/babylon/v3/app/signer"
	cmtcfg "github.com/cometbft/cometbft/config"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
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
	// TODO: Using no-op mempool for EVM until issue with mempool support is resolved
	baseConfig.Mempool.MaxTxs = -1
	// The SDK's default minimum gas price is set to "0.002ubbn" (empty value) inside
	// app.toml, in order to avoid spamming attacks due to transactions with 0 gas price.
	baseConfig.MinGasPrices = fmt.Sprintf("%f%s", appparams.GlobalMinGasPrice, appparams.BaseCoinUnit)
	evmConfig := *evmserverconfig.DefaultEVMConfig()
	evmConfig.EVMChainID = appparams.EVMChainID
	jsonRPCConfig := *evmserverconfig.DefaultJSONRPCConfig()
	jsonRPCConfig.Enable = true
	return &BabylonAppConfig{
		Config:               baseConfig,
		Wasm:                 wasmtypes.DefaultNodeConfig(),
		BtcConfig:            defaultBabylonBtcConfig(),
		BlsConfig:            defaultBabylonBlsConfig(),
		BabylonMempoolConfig: defaultBabylonMempoolConfig(),
		EVM:                  evmConfig,
		JSONRPC:              jsonRPCConfig,
		TLS:                  *evmserverconfig.DefaultTLSConfig(),
	}
}

func DefaultBabylonTemplate() string {
	// manually add evm-chain-id field since cosmos evm v0.3.0 remove this field
	return serverconfig.DefaultConfigTemplate + `
###############################################################################
###                             EVM Configuration                           ###
###############################################################################

[evm]

# EVMChainID defines the EVM compatible chain id
evm-chain-id = "{{ .EVM.EVMChainID }}"

# Tracer defines the 'vm.Tracer' type that the EVM will use when the node is run in
# debug mode. To enable tracing use the '--evm.tracer' flag when starting your node.
# Valid types are: json|struct|access_list|markdown
tracer = "{{ .EVM.Tracer }}"

# MaxTxGasWanted defines the gas wanted for each eth tx returned in ante handler in check tx mode.
max-tx-gas-wanted = {{ .EVM.MaxTxGasWanted }}

# EnablePreimageRecording enables tracking of SHA3 preimages in the VM
cache-preimage = {{ .EVM.EnablePreimageRecording }}

###############################################################################
###                           JSON RPC Configuration                        ###
###############################################################################

[json-rpc]

# Enable defines if the JSONRPC server should be enabled.
enable = {{ .JSONRPC.Enable }}

# Address defines the EVM RPC HTTP server address to bind to.
address = "{{ .JSONRPC.Address }}"

# Address defines the EVM WebSocket server address to bind to.
ws-address = "{{ .JSONRPC.WsAddress }}"

# WSOrigins defines the allowed origins for WebSocket connections.
# Example: ["localhost", "127.0.0.1", "myapp.example.com"]
ws-origins = [{{range $index, $elmt := .JSONRPC.WSOrigins}}{{if $index}}, {{end}}"{{$elmt}}"{{end}}]

# API defines a list of JSON-RPC namespaces that should be enabled
# Example: "eth,txpool,personal,net,debug,web3"
api = "{{range $index, $elmt := .JSONRPC.API}}{{if $index}},{{$elmt}}{{else}}{{$elmt}}{{end}}{{end}}"

# GasCap sets a cap on gas that can be used in eth_call/estimateGas (0=infinite). Default: 25,000,000.
gas-cap = {{ .JSONRPC.GasCap }}

# Allow insecure account unlocking when account-related RPCs are exposed by http
allow-insecure-unlock = {{ .JSONRPC.AllowInsecureUnlock }}

# EVMTimeout is the global timeout for eth_call. Default: 5s.
evm-timeout = "{{ .JSONRPC.EVMTimeout }}"

# TxFeeCap is the global tx-fee cap for send transaction. Default: 1eth.
txfee-cap = {{ .JSONRPC.TxFeeCap }}

# FilterCap sets the global cap for total number of filters that can be created
filter-cap = {{ .JSONRPC.FilterCap }}

# FeeHistoryCap sets the global cap for total number of blocks that can be fetched
feehistory-cap = {{ .JSONRPC.FeeHistoryCap }}

# LogsCap defines the max number of results can be returned from single 'eth_getLogs' query.
logs-cap = {{ .JSONRPC.LogsCap }}

# BlockRangeCap defines the max block range allowed for 'eth_getLogs' query.
block-range-cap = {{ .JSONRPC.BlockRangeCap }}

# HTTPTimeout is the read/write timeout of http json-rpc server.
http-timeout = "{{ .JSONRPC.HTTPTimeout }}"

# HTTPIdleTimeout is the idle timeout of http json-rpc server.
http-idle-timeout = "{{ .JSONRPC.HTTPIdleTimeout }}"

# AllowUnprotectedTxs restricts unprotected (non EIP155 signed) transactions to be submitted via
# the node's RPC when the global parameter is disabled.
allow-unprotected-txs = {{ .JSONRPC.AllowUnprotectedTxs }}

# MaxOpenConnections sets the maximum number of simultaneous connections
# for the server listener.
max-open-connections = {{ .JSONRPC.MaxOpenConnections }}

# EnableIndexer enables the custom transaction indexer for the EVM (ethereum transactions).
enable-indexer = {{ .JSONRPC.EnableIndexer }}

# MetricsAddress defines the EVM Metrics server address to bind to. Pass --metrics in CLI to enable
# Prometheus metrics path: /debug/metrics/prometheus
metrics-address = "{{ .JSONRPC.MetricsAddress }}"

# Upgrade height for fix of revert gas refund logic when transaction reverted.
fix-revert-gas-refund-height = {{ .JSONRPC.FixRevertGasRefundHeight }}

# Enabled profiling in the debug namespace
enable-profiling = {{ .JSONRPC.EnableProfiling }}

###############################################################################
###                             TLS Configuration                           ###
###############################################################################

[tls]

# Certificate path defines the cert.pem file path for the TLS configuration.
certificate-path = "{{ .TLS.CertificatePath }}"

# Key path defines the key.pem file path for the TLS configuration.
key-path = "{{ .TLS.KeyPath }}"
` + wasmtypes.DefaultConfigTemplate() + `
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
