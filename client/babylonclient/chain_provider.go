// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

package babylonclient

import (
	"context"
	"time"

	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
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
	CalculateGas(ctx context.Context, txf tx.Factory, signingKey string, msgs ...sdk.Msg) (txtypes.SimulateResponse, uint64, error)
	BroadcastTx(
		ctx context.Context, // context for tx broadcast
		tx []byte, // raw tx to be broadcast
		asyncCtx context.Context, // context for async wait for block inclusion after successful tx broadcast
		asyncTimeout time.Duration, // timeout for waiting for block inclusion
		asyncCallbacks []func(*RelayerTxResponse, error), // callback for success/fail of the wait for block inclusion
	) error
	WaitForTx(
		ctx context.Context,
		txHash []byte,
		waitTimeout time.Duration,
		callbacks []func(*RelayerTxResponse, error),
	)
	WaitForBlockInclusion(
		ctx context.Context,
		txHash []byte,
		waitTimeout time.Duration,
	) (*sdk.TxResponse, error)
	ChainName() string
	ChainId() string
	ProviderConfig() ProviderConfig
	Key() string
	Address() (string, error)
	Timeout() string
	SetRpcAddr(rpcAddr string) error
}
