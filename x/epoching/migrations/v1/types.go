package v1

import (
	"fmt"

	proto "github.com/cosmos/gogoproto/proto"
)

const (
	DefaultEpochInterval uint64 = 10
)

// Params defines the parameters for the module (v1 legacy version)
// This only contains EpochInterval field from main branch
type Params struct {
	// epoch_interval is the number of consecutive blocks to form an epoch
	EpochInterval uint64 `protobuf:"varint,1,opt,name=epoch_interval,json=epochInterval,proto3" json:"epoch_interval,omitempty" yaml:"epoch_interval"`
}

// NewParams creates a new Params instance
func NewParams(epochInterval uint64) Params {
	return Params{
		EpochInterval: epochInterval,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(DefaultEpochInterval)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateEpochInterval(p.EpochInterval); err != nil {
		return err
	}

	return nil
}

func validateEpochInterval(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v < 2 {
		return fmt.Errorf("epoch interval must be at least 2: %d", v)
	}

	return nil
}

// Proto interface methods for compatibility with codec.BinaryCodec
func (m *Params) Reset()         { *m = Params{} }
func (m *Params) String() string { return proto.CompactTextString(m) }
func (*Params) ProtoMessage()    {}
