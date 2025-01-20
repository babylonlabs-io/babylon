package relayerclient

import (
	"context"
	"fmt"
	"time"

	"github.com/cometbft/cometbft/types"
	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	tendermint "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type BroadcastMode string

const (
	BroadcastModeSingle BroadcastMode = "single"
	BroadcastModeBatch  BroadcastMode = "batch"
)

type ProviderConfig interface {
	NewProvider(log *zap.Logger, homepath string, chainName string) (ChainProvider, error)
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

// loggableEvents is an unexported wrapper type for a slice of RelayerEvent,
// to satisfy the zapcore.ArrayMarshaler interface.
type loggableEvents []RelayerEvent

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface.
func (e RelayerEvent) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("event_type", e.EventType)
	for k, v := range e.Attributes {
		enc.AddString("event_attr: "+k, v)
	}
	return nil
}

// MarshalLogArray satisfies the zapcore.ArrayMarshaler interface.
func (es loggableEvents) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, e := range es {
		enc.AppendObject(e)
	}
	return nil
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface.
func (r RelayerTxResponse) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("height", r.Height)
	enc.AddString("tx_hash", r.TxHash)
	enc.AddString("codespace", r.Codespace)
	enc.AddUint32("code", r.Code)
	enc.AddString("data", r.Data)
	enc.AddArray("events", loggableEvents(r.Events))
	return nil
}

type KeyProvider interface {
	CreateKeystore(path string) error
	KeystoreCreated(path string) bool
	AddKey(name string, coinType uint32, signingAlgorithm string) (output *KeyOutput, err error)
	UseKey(key string) error
	RestoreKey(name, mnemonic string, coinType uint32, signingAlgorithm string) (address string, err error)
	ShowAddress(name string) (address string, err error)
	ListAddresses() (map[string]string, error)
	DeleteKey(name string) error
	KeyExists(name string) bool
	ExportPrivKeyArmor(keyName string) (armor string, err error)
}

type ChainProvider interface {
	QueryProvider
	KeyProvider

	Init(ctx context.Context) error

	SendMessagesToMempool(
		ctx context.Context,
		msgs []RelayerMessage,
		memo string,
		asyncCtx context.Context,
		asyncCallbacks []func(*RelayerTxResponse, error),
	) error

	ChainName() string
	ChainId() string
	Type() string
	ProviderConfig() ProviderConfig
	Key() string
	Address() (string, error)
	Timeout() string
	WaitForNBlocks(ctx context.Context, n int64) error
	Sprint(toPrint proto.Message) (string, error)

	SetRpcAddr(rpcAddr string) error
}

type QueryProvider interface {
	BlockTime(ctx context.Context, height int64) (time.Time, error)
	QueryTx(ctx context.Context, hashHex string) (*RelayerTxResponse, error)
	QueryTxs(ctx context.Context, page, limit int, events []string) ([]*RelayerTxResponse, error)
}

// KeyOutput contains mnemonic and address of key
type KeyOutput struct {
	Mnemonic string `json:"mnemonic" yaml:"mnemonic"`
	Address  string `json:"address" yaml:"address"`
}

// TimeoutHeightError is used during packet validation to inform the PathProcessor
// that the current chain height has exceeded the packet height timeout so that
// a MsgTimeout can be assembled for the counterparty chain.
type TimeoutHeightError struct {
	latestHeight  uint64
	timeoutHeight uint64
}

func (t *TimeoutHeightError) Error() string {
	return fmt.Sprintf("latest height %d is greater than expiration height: %d", t.latestHeight, t.timeoutHeight)
}

func NewTimeoutHeightError(latestHeight, timeoutHeight uint64) *TimeoutHeightError {
	return &TimeoutHeightError{latestHeight, timeoutHeight}
}

// TimeoutTimestampError is used during packet validation to inform the PathProcessor
// that current block timestamp has exceeded the packet timestamp timeout so that
// a MsgTimeout can be assembled for the counterparty chain.
type TimeoutTimestampError struct {
	latestTimestamp  uint64
	timeoutTimestamp uint64
}

func (t *TimeoutTimestampError) Error() string {
	return fmt.Sprintf("latest block timestamp %d is greater than expiration timestamp: %d", t.latestTimestamp, t.timeoutTimestamp)
}

func NewTimeoutTimestampError(latestTimestamp, timeoutTimestamp uint64) *TimeoutTimestampError {
	return &TimeoutTimestampError{latestTimestamp, timeoutTimestamp}
}

type TimeoutOnCloseError struct {
	msg string
}

func (t *TimeoutOnCloseError) Error() string {
	return fmt.Sprintf("packet timeout on close error: %s", t.msg)
}

func NewTimeoutOnCloseError(msg string) *TimeoutOnCloseError {
	return &TimeoutOnCloseError{msg}
}

type TendermintIBCHeader struct {
	SignedHeader      *types.SignedHeader
	ValidatorSet      *types.ValidatorSet
	TrustedValidators *types.ValidatorSet
	TrustedHeight     clienttypes.Height
}

func (h TendermintIBCHeader) Height() uint64 {
	return uint64(h.SignedHeader.Height)
}

func (h TendermintIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return &tendermint.ConsensusState{
		Timestamp:          h.SignedHeader.Time,
		Root:               commitmenttypes.NewMerkleRoot(h.SignedHeader.AppHash),
		NextValidatorsHash: h.SignedHeader.NextValidatorsHash,
	}
}

func (h TendermintIBCHeader) NextValidatorsHash() []byte {
	return h.SignedHeader.NextValidatorsHash
}

func (h TendermintIBCHeader) TMHeader() (*tendermint.Header, error) {
	valSet, err := h.ValidatorSet.ToProto()
	if err != nil {
		return nil, err
	}

	trustedVals, err := h.TrustedValidators.ToProto()
	if err != nil {
		return nil, err
	}

	return &tendermint.Header{
		SignedHeader:      h.SignedHeader.ToProto(),
		ValidatorSet:      valSet,
		TrustedHeight:     h.TrustedHeight,
		TrustedValidators: trustedVals,
	}, nil
}

type ExtensionOption struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
