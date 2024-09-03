package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/babylonlabs-io/finality-provider/cosmwasmclient/config"
	"github.com/cosmos/btcutil/bech32"
)

type CosmwasmConfig struct {
	Key                       string        `long:"key" description:"name of the key to sign transactions with"`
	ChainID                   string        `long:"chain-id" description:"chain id of the chain to connect to"`
	RPCAddr                   string        `long:"rpc-address" description:"address of the rpc server to connect to"`
	GRPCAddr                  string        `long:"grpc-address" description:"address of the grpc server to connect to"`
	AccountPrefix             string        `long:"acc-prefix" description:"account prefix to use for addresses"`
	KeyringBackend            string        `long:"keyring-type" description:"type of keyring to use"`
	GasAdjustment             float64       `long:"gas-adjustment" description:"adjustment factor when using gas estimation"`
	GasPrices                 string        `long:"gas-prices" description:"comma separated minimum gas prices to accept for transactions"`
	KeyDirectory              string        `long:"key-dir" description:"directory to store keys in"`
	Debug                     bool          `long:"debug" description:"flag to print debug output"`
	Timeout                   time.Duration `long:"timeout" description:"client timeout when doing queries"`
	BlockTimeout              time.Duration `long:"block-timeout" description:"block timeout when waiting for block events"`
	OutputFormat              string        `long:"output-format" description:"default output when printint responses"`
	SignModeStr               string        `long:"sign-mode" description:"sign mode to use"`
	BtcStakingContractAddress string        `long:"btc-staking-contract-address" description:"address of the BTC staking contract"`
}

func (cfg *CosmwasmConfig) Validate() error {
	if _, err := url.Parse(cfg.RPCAddr); err != nil {
		return fmt.Errorf("rpc-addr is not correctly formatted: %w", err)
	}

	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if cfg.BlockTimeout < 0 {
		return fmt.Errorf("block-timeout can't be negative")
	}

	_, _, err := bech32.Decode(cfg.BtcStakingContractAddress, len(cfg.BtcStakingContractAddress))
	if err != nil {
		return fmt.Errorf("babylon-contract-address: invalid bech32 address: %w", err)
	}
	if !strings.HasPrefix(cfg.BtcStakingContractAddress, cfg.AccountPrefix) {
		return fmt.Errorf("babylon-contract-address: invalid address prefix: %w", err)
	}
	return nil
}

func DefaultCosmwasmConfig() *CosmwasmConfig {
	return &CosmwasmConfig{
		Key:                       "validator",
		ChainID:                   "wasmd-test",
		RPCAddr:                   "http://localhost:2990",
		GRPCAddr:                  "https://localhost:9090",
		AccountPrefix:             "wasm",
		KeyringBackend:            "test",
		GasAdjustment:             1.3,
		GasPrices:                 "1ustake",
		Debug:                     true,
		Timeout:                   20 * time.Second,
		BlockTimeout:              1 * time.Minute,
		OutputFormat:              "direct",
		SignModeStr:               "",
		BtcStakingContractAddress: "",
	}
}

func (cfg *CosmwasmConfig) ToQueryClientConfig() *config.CosmwasmConfig {
	return &config.CosmwasmConfig{
		Key:              cfg.Key,
		ChainID:          cfg.ChainID,
		RPCAddr:          cfg.RPCAddr,
		AccountPrefix:    cfg.AccountPrefix,
		KeyringBackend:   cfg.KeyringBackend,
		GasAdjustment:    cfg.GasAdjustment,
		GasPrices:        cfg.GasPrices,
		KeyDirectory:     cfg.KeyDirectory,
		Debug:            cfg.Debug,
		Timeout:          cfg.Timeout,
		BlockTimeout:     cfg.BlockTimeout,
		OutputFormat:     cfg.OutputFormat,
		SignModeStr:      cfg.SignModeStr,
		SubmitterAddress: "",
	}
}
