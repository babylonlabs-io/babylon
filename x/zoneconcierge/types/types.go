package types

import "time"

type HeaderInfo struct {
	ClientId string
	ChainId  string
	AppHash  []byte
	Height   uint64
	Time     time.Time
}

var FailedToSendPacket = false
