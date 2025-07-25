package types

import (
	"fmt"
)

const (
	DefaultIbcPacketTimeoutSeconds uint32 = 60 * 60 * 24       // 24 hours
	MaxIbcPacketTimeoutSeconds     uint32 = 60 * 60 * 24 * 365 // 1 year
	DefaultMaxHeadersPerPacket     uint32 = 100                // Max BTC headers per IBC packet
)

// NewParams creates a new Params instance
func NewParams(ibcPacketTimeoutSeconds uint32, maxHeadersPerPacket uint32) Params {
	if maxHeadersPerPacket == 0 {
		maxHeadersPerPacket = DefaultMaxHeadersPerPacket
	}
	return Params{
		IbcPacketTimeoutSeconds: ibcPacketTimeoutSeconds,
		MaxHeadersPerPacket:     maxHeadersPerPacket,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(DefaultIbcPacketTimeoutSeconds, DefaultMaxHeadersPerPacket)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.IbcPacketTimeoutSeconds == 0 {
		return fmt.Errorf("IbcPacketTimeoutSeconds must be positive")
	}
	if p.IbcPacketTimeoutSeconds > MaxIbcPacketTimeoutSeconds {
		return fmt.Errorf("IbcPacketTimeoutSeconds must be no larger than %d", MaxIbcPacketTimeoutSeconds)
	}
	if p.MaxHeadersPerPacket == 0 {
		return fmt.Errorf("MaxHeadersPerPacket must be positive")
	}

	return nil
}
