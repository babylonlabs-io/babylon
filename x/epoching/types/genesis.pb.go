// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: babylon/epoching/v1/genesis.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// GenesisState defines the epoching module's genesis state.
type GenesisState struct {
	// params are the current params of the state.
	Params Params `protobuf:"bytes,1,opt,name=params,proto3" json:"params"`
	// epochs contains all the epochs info
	Epochs []*Epoch `protobuf:"bytes,2,rep,name=epochs,proto3" json:"epochs,omitempty"`
	// queues contains all the epochs' queue
	Queues []*EpochQueue `protobuf:"bytes,3,rep,name=queues,proto3" json:"queues,omitempty"`
	// validator_sets is a slice containing all the
	// stored epochs' validator sets
	ValidatorSets []*EpochValidatorSet `protobuf:"bytes,4,rep,name=validator_sets,json=validatorSets,proto3" json:"validator_sets,omitempty"`
	// slashed_validator_sets is a slice containing all the
	// stored epochs' slashed validator sets
	SlashedValidatorSets []*EpochValidatorSet `protobuf:"bytes,5,rep,name=slashed_validator_sets,json=slashedValidatorSets,proto3" json:"slashed_validator_sets,omitempty"`
	// validators_lifecycle contains the lifecyle of all validators
	ValidatorsLifecycle []*ValidatorLifecycle `protobuf:"bytes,6,rep,name=validators_lifecycle,json=validatorsLifecycle,proto3" json:"validators_lifecycle,omitempty"`
	// delegations_lifecycle contains the lifecyle of all delegations
	DelegationsLifecycle []*DelegationLifecycle `protobuf:"bytes,7,rep,name=delegations_lifecycle,json=delegationsLifecycle,proto3" json:"delegations_lifecycle,omitempty"`
}

func (m *GenesisState) Reset()         { *m = GenesisState{} }
func (m *GenesisState) String() string { return proto.CompactTextString(m) }
func (*GenesisState) ProtoMessage()    {}
func (*GenesisState) Descriptor() ([]byte, []int) {
	return fileDescriptor_2ef836361c424501, []int{0}
}
func (m *GenesisState) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GenesisState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GenesisState.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GenesisState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GenesisState.Merge(m, src)
}
func (m *GenesisState) XXX_Size() int {
	return m.Size()
}
func (m *GenesisState) XXX_DiscardUnknown() {
	xxx_messageInfo_GenesisState.DiscardUnknown(m)
}

var xxx_messageInfo_GenesisState proto.InternalMessageInfo

func (m *GenesisState) GetParams() Params {
	if m != nil {
		return m.Params
	}
	return Params{}
}

func (m *GenesisState) GetEpochs() []*Epoch {
	if m != nil {
		return m.Epochs
	}
	return nil
}

func (m *GenesisState) GetQueues() []*EpochQueue {
	if m != nil {
		return m.Queues
	}
	return nil
}

func (m *GenesisState) GetValidatorSets() []*EpochValidatorSet {
	if m != nil {
		return m.ValidatorSets
	}
	return nil
}

func (m *GenesisState) GetSlashedValidatorSets() []*EpochValidatorSet {
	if m != nil {
		return m.SlashedValidatorSets
	}
	return nil
}

func (m *GenesisState) GetValidatorsLifecycle() []*ValidatorLifecycle {
	if m != nil {
		return m.ValidatorsLifecycle
	}
	return nil
}

func (m *GenesisState) GetDelegationsLifecycle() []*DelegationLifecycle {
	if m != nil {
		return m.DelegationsLifecycle
	}
	return nil
}

// EpochQueue defines a genesis state entry for
// the epochs' message queue
type EpochQueue struct {
	// epoch_number is the epoch's identifier
	EpochNumber uint64 `protobuf:"varint,1,opt,name=epoch_number,json=epochNumber,proto3" json:"epoch_number,omitempty"`
	// msgs is a slice containing all the epochs' queued messages
	Msgs []*QueuedMessage `protobuf:"bytes,2,rep,name=msgs,proto3" json:"msgs,omitempty"`
}

func (m *EpochQueue) Reset()         { *m = EpochQueue{} }
func (m *EpochQueue) String() string { return proto.CompactTextString(m) }
func (*EpochQueue) ProtoMessage()    {}
func (*EpochQueue) Descriptor() ([]byte, []int) {
	return fileDescriptor_2ef836361c424501, []int{1}
}
func (m *EpochQueue) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *EpochQueue) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_EpochQueue.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *EpochQueue) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EpochQueue.Merge(m, src)
}
func (m *EpochQueue) XXX_Size() int {
	return m.Size()
}
func (m *EpochQueue) XXX_DiscardUnknown() {
	xxx_messageInfo_EpochQueue.DiscardUnknown(m)
}

var xxx_messageInfo_EpochQueue proto.InternalMessageInfo

func (m *EpochQueue) GetEpochNumber() uint64 {
	if m != nil {
		return m.EpochNumber
	}
	return 0
}

func (m *EpochQueue) GetMsgs() []*QueuedMessage {
	if m != nil {
		return m.Msgs
	}
	return nil
}

// EpochValidatorSet contains the epoch number and the validators corresponding
// to that epoch number
type EpochValidatorSet struct {
	// epoch_number is the epoch's identifier
	EpochNumber uint64 `protobuf:"varint,1,opt,name=epoch_number,json=epochNumber,proto3" json:"epoch_number,omitempty"`
	// validators is a slice containing the validators of the
	// epoch's validator set
	Validators []*Validator `protobuf:"bytes,2,rep,name=validators,proto3" json:"validators,omitempty"`
}

func (m *EpochValidatorSet) Reset()         { *m = EpochValidatorSet{} }
func (m *EpochValidatorSet) String() string { return proto.CompactTextString(m) }
func (*EpochValidatorSet) ProtoMessage()    {}
func (*EpochValidatorSet) Descriptor() ([]byte, []int) {
	return fileDescriptor_2ef836361c424501, []int{2}
}
func (m *EpochValidatorSet) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *EpochValidatorSet) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_EpochValidatorSet.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *EpochValidatorSet) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EpochValidatorSet.Merge(m, src)
}
func (m *EpochValidatorSet) XXX_Size() int {
	return m.Size()
}
func (m *EpochValidatorSet) XXX_DiscardUnknown() {
	xxx_messageInfo_EpochValidatorSet.DiscardUnknown(m)
}

var xxx_messageInfo_EpochValidatorSet proto.InternalMessageInfo

func (m *EpochValidatorSet) GetEpochNumber() uint64 {
	if m != nil {
		return m.EpochNumber
	}
	return 0
}

func (m *EpochValidatorSet) GetValidators() []*Validator {
	if m != nil {
		return m.Validators
	}
	return nil
}

func init() {
	proto.RegisterType((*GenesisState)(nil), "babylon.epoching.v1.GenesisState")
	proto.RegisterType((*EpochQueue)(nil), "babylon.epoching.v1.EpochQueue")
	proto.RegisterType((*EpochValidatorSet)(nil), "babylon.epoching.v1.EpochValidatorSet")
}

func init() { proto.RegisterFile("babylon/epoching/v1/genesis.proto", fileDescriptor_2ef836361c424501) }

var fileDescriptor_2ef836361c424501 = []byte{
	// 453 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x93, 0xcb, 0x8e, 0xd3, 0x30,
	0x14, 0x86, 0x1b, 0x26, 0x04, 0xe9, 0x74, 0x40, 0xc2, 0x53, 0x50, 0x54, 0xa4, 0x4c, 0x27, 0x0b,
	0xe8, 0x86, 0x44, 0x53, 0x6e, 0x62, 0xc3, 0x62, 0x04, 0x62, 0xc3, 0x70, 0xf1, 0x48, 0xb3, 0x18,
	0x81, 0x2a, 0xa7, 0x3d, 0xb8, 0x91, 0xd2, 0xb8, 0xc4, 0x4e, 0x44, 0xdf, 0x82, 0x27, 0xe0, 0x79,
	0x66, 0x39, 0x4b, 0x56, 0x08, 0xb5, 0x2f, 0x82, 0xea, 0x3a, 0x17, 0x20, 0x83, 0x60, 0x67, 0x1f,
	0x7f, 0xff, 0x77, 0xac, 0x23, 0x1b, 0x0e, 0x22, 0x16, 0x2d, 0x13, 0x91, 0x86, 0xb8, 0x10, 0x93,
	0x59, 0x9c, 0xf2, 0xb0, 0x38, 0x0c, 0x39, 0xa6, 0x28, 0x63, 0x19, 0x2c, 0x32, 0xa1, 0x04, 0xd9,
	0x33, 0x48, 0x50, 0x22, 0x41, 0x71, 0xd8, 0xef, 0x71, 0xc1, 0x85, 0x3e, 0x0f, 0x37, 0xab, 0x2d,
	0xda, 0x1f, 0xb4, 0xd9, 0x16, 0x2c, 0x63, 0x73, 0x23, 0xeb, 0xfb, 0x6d, 0x44, 0x25, 0xd6, 0x8c,
	0xff, 0xd5, 0x86, 0xdd, 0x97, 0xdb, 0x2b, 0x9c, 0x28, 0xa6, 0x90, 0x3c, 0x05, 0x67, 0x2b, 0x71,
	0xad, 0x81, 0x35, 0xec, 0x8e, 0xee, 0x04, 0x2d, 0x57, 0x0a, 0xde, 0x6a, 0xe4, 0xc8, 0x3e, 0xff,
	0xbe, 0xdf, 0xa1, 0x26, 0x40, 0x46, 0xe0, 0x68, 0x46, 0xba, 0x57, 0x06, 0x3b, 0xc3, 0xee, 0xa8,
	0xdf, 0x1a, 0x7d, 0xb1, 0x59, 0x53, 0x43, 0x92, 0x27, 0xe0, 0x7c, 0xca, 0x31, 0x47, 0xe9, 0xee,
	0xe8, 0xcc, 0xfe, 0xe5, 0x99, 0x77, 0x1b, 0x8e, 0x1a, 0x9c, 0x1c, 0xc3, 0x8d, 0x82, 0x25, 0xf1,
	0x94, 0x29, 0x91, 0x8d, 0x25, 0x2a, 0xe9, 0xda, 0x5a, 0x70, 0xf7, 0x72, 0xc1, 0x69, 0xc9, 0x9f,
	0xa0, 0xa2, 0xd7, 0x8b, 0xc6, 0x4e, 0x92, 0xf7, 0x70, 0x5b, 0x26, 0x4c, 0xce, 0x70, 0x3a, 0xfe,
	0x4d, 0x7b, 0xf5, 0xbf, 0xb4, 0x3d, 0x63, 0x39, 0xfd, 0xc5, 0x7e, 0x06, 0xbd, 0xca, 0x2a, 0xc7,
	0x49, 0xfc, 0x11, 0x27, 0xcb, 0x49, 0x82, 0xae, 0xa3, 0xdd, 0xf7, 0x5a, 0xdd, 0x95, 0xe1, 0x55,
	0x89, 0xd3, 0xbd, 0x5a, 0x52, 0x15, 0xc9, 0x07, 0xb8, 0x35, 0xc5, 0x04, 0x39, 0x53, 0xb1, 0x48,
	0x9b, 0xf2, 0x6b, 0x5a, 0x3e, 0x6c, 0x95, 0x3f, 0xaf, 0x12, 0xb5, 0xbd, 0xd7, 0xd0, 0x54, 0x55,
	0x9f, 0x03, 0xd4, 0xd3, 0x27, 0x07, 0xb0, 0xab, 0x35, 0xe3, 0x34, 0x9f, 0x47, 0x98, 0xe9, 0x37,
	0x62, 0xd3, 0xae, 0xae, 0xbd, 0xd6, 0x25, 0xf2, 0x18, 0xec, 0xb9, 0xe4, 0xe5, 0x1b, 0xf0, 0x5b,
	0xdb, 0x6b, 0xd9, 0xf4, 0x18, 0xa5, 0x64, 0x1c, 0xa9, 0xe6, 0xfd, 0x02, 0x6e, 0xfe, 0x31, 0xce,
	0x7f, 0xe9, 0xf7, 0x0c, 0xa0, 0x1e, 0x8b, 0xe9, 0xea, 0xfd, 0x7d, 0xa2, 0xb4, 0x91, 0x38, 0x7a,
	0x73, 0xbe, 0xf2, 0xac, 0x8b, 0x95, 0x67, 0xfd, 0x58, 0x79, 0xd6, 0x97, 0xb5, 0xd7, 0xb9, 0x58,
	0x7b, 0x9d, 0x6f, 0x6b, 0xaf, 0x73, 0xf6, 0x88, 0xc7, 0x6a, 0x96, 0x47, 0xc1, 0x44, 0xcc, 0x43,
	0xe3, 0x4b, 0x58, 0x24, 0xef, 0xc7, 0xa2, 0xdc, 0x86, 0xc5, 0xc3, 0xf0, 0x73, 0xfd, 0xbd, 0xd4,
	0x72, 0x81, 0x32, 0x72, 0xf4, 0xcf, 0x7a, 0xf0, 0x33, 0x00, 0x00, 0xff, 0xff, 0xd7, 0x3b, 0x9e,
	0xb9, 0xef, 0x03, 0x00, 0x00,
}

func (m *GenesisState) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GenesisState) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GenesisState) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.DelegationsLifecycle) > 0 {
		for iNdEx := len(m.DelegationsLifecycle) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.DelegationsLifecycle[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x3a
		}
	}
	if len(m.ValidatorsLifecycle) > 0 {
		for iNdEx := len(m.ValidatorsLifecycle) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.ValidatorsLifecycle[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x32
		}
	}
	if len(m.SlashedValidatorSets) > 0 {
		for iNdEx := len(m.SlashedValidatorSets) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.SlashedValidatorSets[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x2a
		}
	}
	if len(m.ValidatorSets) > 0 {
		for iNdEx := len(m.ValidatorSets) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.ValidatorSets[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x22
		}
	}
	if len(m.Queues) > 0 {
		for iNdEx := len(m.Queues) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Queues[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x1a
		}
	}
	if len(m.Epochs) > 0 {
		for iNdEx := len(m.Epochs) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Epochs[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	{
		size, err := m.Params.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintGenesis(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func (m *EpochQueue) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *EpochQueue) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *EpochQueue) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Msgs) > 0 {
		for iNdEx := len(m.Msgs) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Msgs[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if m.EpochNumber != 0 {
		i = encodeVarintGenesis(dAtA, i, uint64(m.EpochNumber))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *EpochValidatorSet) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *EpochValidatorSet) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *EpochValidatorSet) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Validators) > 0 {
		for iNdEx := len(m.Validators) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Validators[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if m.EpochNumber != 0 {
		i = encodeVarintGenesis(dAtA, i, uint64(m.EpochNumber))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintGenesis(dAtA []byte, offset int, v uint64) int {
	offset -= sovGenesis(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *GenesisState) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.Params.Size()
	n += 1 + l + sovGenesis(uint64(l))
	if len(m.Epochs) > 0 {
		for _, e := range m.Epochs {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	if len(m.Queues) > 0 {
		for _, e := range m.Queues {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	if len(m.ValidatorSets) > 0 {
		for _, e := range m.ValidatorSets {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	if len(m.SlashedValidatorSets) > 0 {
		for _, e := range m.SlashedValidatorSets {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	if len(m.ValidatorsLifecycle) > 0 {
		for _, e := range m.ValidatorsLifecycle {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	if len(m.DelegationsLifecycle) > 0 {
		for _, e := range m.DelegationsLifecycle {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	return n
}

func (m *EpochQueue) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.EpochNumber != 0 {
		n += 1 + sovGenesis(uint64(m.EpochNumber))
	}
	if len(m.Msgs) > 0 {
		for _, e := range m.Msgs {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	return n
}

func (m *EpochValidatorSet) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.EpochNumber != 0 {
		n += 1 + sovGenesis(uint64(m.EpochNumber))
	}
	if len(m.Validators) > 0 {
		for _, e := range m.Validators {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	return n
}

func sovGenesis(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozGenesis(x uint64) (n int) {
	return sovGenesis(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *GenesisState) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenesis
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: GenesisState: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GenesisState: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Params", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Params.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Epochs", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Epochs = append(m.Epochs, &Epoch{})
			if err := m.Epochs[len(m.Epochs)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Queues", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Queues = append(m.Queues, &EpochQueue{})
			if err := m.Queues[len(m.Queues)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ValidatorSets", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ValidatorSets = append(m.ValidatorSets, &EpochValidatorSet{})
			if err := m.ValidatorSets[len(m.ValidatorSets)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SlashedValidatorSets", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SlashedValidatorSets = append(m.SlashedValidatorSets, &EpochValidatorSet{})
			if err := m.SlashedValidatorSets[len(m.SlashedValidatorSets)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ValidatorsLifecycle", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ValidatorsLifecycle = append(m.ValidatorsLifecycle, &ValidatorLifecycle{})
			if err := m.ValidatorsLifecycle[len(m.ValidatorsLifecycle)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DelegationsLifecycle", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DelegationsLifecycle = append(m.DelegationsLifecycle, &DelegationLifecycle{})
			if err := m.DelegationsLifecycle[len(m.DelegationsLifecycle)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenesis(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenesis
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *EpochQueue) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenesis
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: EpochQueue: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: EpochQueue: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EpochNumber", wireType)
			}
			m.EpochNumber = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.EpochNumber |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Msgs", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Msgs = append(m.Msgs, &QueuedMessage{})
			if err := m.Msgs[len(m.Msgs)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenesis(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenesis
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *EpochValidatorSet) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenesis
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: EpochValidatorSet: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: EpochValidatorSet: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EpochNumber", wireType)
			}
			m.EpochNumber = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.EpochNumber |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Validators", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Validators = append(m.Validators, &Validator{})
			if err := m.Validators[len(m.Validators)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenesis(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenesis
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipGenesis(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowGenesis
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthGenesis
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupGenesis
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthGenesis
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthGenesis        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowGenesis          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupGenesis = fmt.Errorf("proto: unexpected end of group")
)
