// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

package babylonclient

import (
	"context"
)

type BroadcastMode string

const (
	BroadcastModeSingle BroadcastMode = "single"
	BroadcastModeBatch  BroadcastMode = "batch"
)

type ProviderConfig interface {
	NewProvider(homepath string, chainName string) (ChainProvider, error)
	Validate() error
	BroadcastMode() BroadcastMode
}

type RelayerMessage interface {
	Type() string
	MsgBytes() ([]byte, error)
}

type RelayerTxResponse struct {
	Height    int64
	TxHash    string
	Codespace string
	Code      uint32
	Data      string
	Events    []RelayerEvent
}

type RelayerEvent struct {
	EventType  string
	Attributes map[string]string
}

type ChainProvider interface {
	Init() error
	SendMessagesToMempool(
		ctx context.Context,
		msgs []RelayerMessage,
		memo string,
		asyncCtx context.Context,
		asyncCallbacks []func(*RelayerTxResponse, error),
	) error
	ChainName() string
	ChainId() string
	ProviderConfig() ProviderConfig
	Key() string
	Address() (string, error)
	Timeout() string
	SetRpcAddr(rpcAddr string) error
}
