package relayerclient

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

const (
	ErrTimeoutAfterWaitingForTxBroadcast _err = "timed out after waiting for tx to get included in the block"
)

type _err string

func (e _err) Error() string { return string(e) }

type intoAny interface {
	AsAny() *codectypes.Any
}
