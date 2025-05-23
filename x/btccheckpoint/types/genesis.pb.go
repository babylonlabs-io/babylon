// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: babylon/btccheckpoint/v1/genesis.proto

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

// GenesisState defines the btccheckpoint module's genesis state.
type GenesisState struct {
	// params the current params of the state.
	Params Params `protobuf:"bytes,1,opt,name=params,proto3" json:"params"`
	// the last finalized epoch number
	LastFinalizedEpochNumber uint64 `protobuf:"varint,2,opt,name=last_finalized_epoch_number,json=lastFinalizedEpochNumber,proto3" json:"last_finalized_epoch_number,omitempty"`
	// Epochs data for each stored epoch
	Epochs []EpochEntry `protobuf:"bytes,3,rep,name=epochs,proto3" json:"epochs"`
	// Submission data for each stored submission key
	Submissions []SubmissionEntry `protobuf:"bytes,4,rep,name=submissions,proto3" json:"submissions"`
}

func (m *GenesisState) Reset()         { *m = GenesisState{} }
func (m *GenesisState) String() string { return proto.CompactTextString(m) }
func (*GenesisState) ProtoMessage()    {}
func (*GenesisState) Descriptor() ([]byte, []int) {
	return fileDescriptor_9776220697c13f63, []int{0}
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

func (m *GenesisState) GetLastFinalizedEpochNumber() uint64 {
	if m != nil {
		return m.LastFinalizedEpochNumber
	}
	return 0
}

func (m *GenesisState) GetEpochs() []EpochEntry {
	if m != nil {
		return m.Epochs
	}
	return nil
}

func (m *GenesisState) GetSubmissions() []SubmissionEntry {
	if m != nil {
		return m.Submissions
	}
	return nil
}

// EpochEntry represents data for a specific epoch number.
type EpochEntry struct {
	// Epoch number
	EpochNumber uint64 `protobuf:"varint,1,opt,name=epoch_number,json=epochNumber,proto3" json:"epoch_number,omitempty"`
	// The epoch data
	Data *EpochData `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (m *EpochEntry) Reset()         { *m = EpochEntry{} }
func (m *EpochEntry) String() string { return proto.CompactTextString(m) }
func (*EpochEntry) ProtoMessage()    {}
func (*EpochEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_9776220697c13f63, []int{1}
}
func (m *EpochEntry) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *EpochEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_EpochEntry.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *EpochEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EpochEntry.Merge(m, src)
}
func (m *EpochEntry) XXX_Size() int {
	return m.Size()
}
func (m *EpochEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_EpochEntry.DiscardUnknown(m)
}

var xxx_messageInfo_EpochEntry proto.InternalMessageInfo

func (m *EpochEntry) GetEpochNumber() uint64 {
	if m != nil {
		return m.EpochNumber
	}
	return 0
}

func (m *EpochEntry) GetData() *EpochData {
	if m != nil {
		return m.Data
	}
	return nil
}

// SubmissionEntry represents data for a submission for
// a specific submission key.
type SubmissionEntry struct {
	// Epoch number
	SubmissionKey *SubmissionKey `protobuf:"bytes,1,opt,name=submission_key,json=submissionKey,proto3" json:"submission_key,omitempty"`
	// The submission data corresponding to the submission key
	Data *SubmissionData `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (m *SubmissionEntry) Reset()         { *m = SubmissionEntry{} }
func (m *SubmissionEntry) String() string { return proto.CompactTextString(m) }
func (*SubmissionEntry) ProtoMessage()    {}
func (*SubmissionEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_9776220697c13f63, []int{2}
}
func (m *SubmissionEntry) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SubmissionEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SubmissionEntry.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SubmissionEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SubmissionEntry.Merge(m, src)
}
func (m *SubmissionEntry) XXX_Size() int {
	return m.Size()
}
func (m *SubmissionEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_SubmissionEntry.DiscardUnknown(m)
}

var xxx_messageInfo_SubmissionEntry proto.InternalMessageInfo

func (m *SubmissionEntry) GetSubmissionKey() *SubmissionKey {
	if m != nil {
		return m.SubmissionKey
	}
	return nil
}

func (m *SubmissionEntry) GetData() *SubmissionData {
	if m != nil {
		return m.Data
	}
	return nil
}

func init() {
	proto.RegisterType((*GenesisState)(nil), "babylon.btccheckpoint.v1.GenesisState")
	proto.RegisterType((*EpochEntry)(nil), "babylon.btccheckpoint.v1.EpochEntry")
	proto.RegisterType((*SubmissionEntry)(nil), "babylon.btccheckpoint.v1.SubmissionEntry")
}

func init() {
	proto.RegisterFile("babylon/btccheckpoint/v1/genesis.proto", fileDescriptor_9776220697c13f63)
}

var fileDescriptor_9776220697c13f63 = []byte{
	// 406 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x92, 0x4f, 0xcb, 0xd3, 0x30,
	0x1c, 0xc7, 0x9b, 0xe7, 0x29, 0x3b, 0xa4, 0x8f, 0x0a, 0xc1, 0x43, 0x99, 0x50, 0xeb, 0xfc, 0x57,
	0x41, 0x5b, 0x36, 0x05, 0x41, 0xd4, 0xc3, 0x70, 0x7a, 0x10, 0x86, 0x76, 0x9e, 0xbc, 0x94, 0xa4,
	0x8b, 0x6d, 0x58, 0xdb, 0x94, 0x26, 0x1b, 0xd6, 0x57, 0xe1, 0x3b, 0xf0, 0xe2, 0x8b, 0xd9, 0x71,
	0x47, 0x4f, 0x22, 0xdb, 0x1b, 0x91, 0xa5, 0x9d, 0x6b, 0x07, 0xc5, 0xdd, 0x1a, 0xfa, 0xf9, 0x7d,
	0x7e, 0xdf, 0x2f, 0x09, 0x7c, 0x40, 0x30, 0x29, 0x13, 0x9e, 0x79, 0x44, 0x86, 0x61, 0x4c, 0xc3,
	0x45, 0xce, 0x59, 0x26, 0xbd, 0xd5, 0xd0, 0x8b, 0x68, 0x46, 0x05, 0x13, 0x6e, 0x5e, 0x70, 0xc9,
	0x91, 0x59, 0x73, 0x6e, 0x8b, 0x73, 0x57, 0xc3, 0xfe, 0xcd, 0x88, 0x47, 0x5c, 0x41, 0xde, 0xfe,
	0xab, 0xe2, 0xfb, 0xf7, 0x3b, 0xbd, 0x39, 0x2e, 0x70, 0x5a, 0x6b, 0xfb, 0x8f, 0x3b, 0xb1, 0xf6,
	0x1e, 0x45, 0x0f, 0x7e, 0x5e, 0xc0, 0xab, 0x77, 0x55, 0xac, 0x99, 0xc4, 0x92, 0xa2, 0xd7, 0xb0,
	0x57, 0xe9, 0x4c, 0x60, 0x03, 0xc7, 0x18, 0xd9, 0x6e, 0x57, 0x4c, 0xf7, 0x83, 0xe2, 0xc6, 0xfa,
	0xfa, 0xf7, 0x6d, 0xcd, 0xaf, 0xa7, 0xd0, 0x2b, 0x78, 0x2b, 0xc1, 0x42, 0x06, 0x5f, 0x58, 0x86,
	0x13, 0xf6, 0x8d, 0xce, 0x03, 0x9a, 0xf3, 0x30, 0x0e, 0xb2, 0x65, 0x4a, 0x68, 0x61, 0x5e, 0xd8,
	0xc0, 0xd1, 0x7d, 0x73, 0x8f, 0xbc, 0x3d, 0x10, 0x93, 0x3d, 0x30, 0x55, 0xff, 0xd1, 0x18, 0xf6,
	0x14, 0x2f, 0xcc, 0x4b, 0xfb, 0xd2, 0x31, 0x46, 0xf7, 0xba, 0xd7, 0xab, 0xb1, 0x49, 0x26, 0x8b,
	0xf2, 0x10, 0xa1, 0x9a, 0x44, 0x1f, 0xa1, 0x21, 0x96, 0x24, 0x65, 0x42, 0x30, 0x9e, 0x09, 0x53,
	0x57, 0xa2, 0x47, 0xdd, 0xa2, 0xd9, 0x3f, 0xb8, 0x69, 0x6b, 0x3a, 0x06, 0x31, 0x84, 0xc7, 0x75,
	0xe8, 0x0e, 0xbc, 0x6a, 0x95, 0x02, 0xaa, 0x94, 0x41, 0x1b, 0x3d, 0x9e, 0x43, 0x7d, 0x8e, 0x25,
	0x56, 0x7d, 0x8d, 0xd1, 0xdd, 0xff, 0xb4, 0x78, 0x83, 0x25, 0xf6, 0xd5, 0xc0, 0xe0, 0x07, 0x80,
	0x37, 0x4e, 0x02, 0xa1, 0x29, 0xbc, 0x7e, 0x0c, 0x13, 0x2c, 0x68, 0x59, 0xdf, 0xcd, 0xc3, 0x73,
	0x3a, 0xbd, 0xa7, 0xa5, 0x7f, 0x4d, 0x34, 0x8f, 0xe8, 0x65, 0x2b, 0x9c, 0x73, 0x8e, 0xe5, 0x98,
	0x70, 0xfc, 0x69, 0xbd, 0xb5, 0xc0, 0x66, 0x6b, 0x81, 0x3f, 0x5b, 0x0b, 0x7c, 0xdf, 0x59, 0xda,
	0x66, 0x67, 0x69, 0xbf, 0x76, 0x96, 0xf6, 0xf9, 0x45, 0xc4, 0x64, 0xbc, 0x24, 0x6e, 0xc8, 0x53,
	0xaf, 0x76, 0x26, 0x98, 0x88, 0x27, 0x8c, 0x1f, 0x8e, 0xde, 0xea, 0x99, 0xf7, 0xf5, 0xe4, 0x65,
	0xca, 0x32, 0xa7, 0x82, 0xf4, 0xd4, 0x7b, 0x7c, 0xfa, 0x37, 0x00, 0x00, 0xff, 0xff, 0x23, 0xc3,
	0x81, 0xf6, 0x3e, 0x03, 0x00, 0x00,
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
	if len(m.Submissions) > 0 {
		for iNdEx := len(m.Submissions) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Submissions[iNdEx].MarshalToSizedBuffer(dAtA[:i])
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
			dAtA[i] = 0x1a
		}
	}
	if m.LastFinalizedEpochNumber != 0 {
		i = encodeVarintGenesis(dAtA, i, uint64(m.LastFinalizedEpochNumber))
		i--
		dAtA[i] = 0x10
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

func (m *EpochEntry) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *EpochEntry) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *EpochEntry) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Data != nil {
		{
			size, err := m.Data.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGenesis(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	if m.EpochNumber != 0 {
		i = encodeVarintGenesis(dAtA, i, uint64(m.EpochNumber))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *SubmissionEntry) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SubmissionEntry) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SubmissionEntry) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Data != nil {
		{
			size, err := m.Data.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGenesis(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	if m.SubmissionKey != nil {
		{
			size, err := m.SubmissionKey.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGenesis(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
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
	if m.LastFinalizedEpochNumber != 0 {
		n += 1 + sovGenesis(uint64(m.LastFinalizedEpochNumber))
	}
	if len(m.Epochs) > 0 {
		for _, e := range m.Epochs {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	if len(m.Submissions) > 0 {
		for _, e := range m.Submissions {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	return n
}

func (m *EpochEntry) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.EpochNumber != 0 {
		n += 1 + sovGenesis(uint64(m.EpochNumber))
	}
	if m.Data != nil {
		l = m.Data.Size()
		n += 1 + l + sovGenesis(uint64(l))
	}
	return n
}

func (m *SubmissionEntry) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.SubmissionKey != nil {
		l = m.SubmissionKey.Size()
		n += 1 + l + sovGenesis(uint64(l))
	}
	if m.Data != nil {
		l = m.Data.Size()
		n += 1 + l + sovGenesis(uint64(l))
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
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field LastFinalizedEpochNumber", wireType)
			}
			m.LastFinalizedEpochNumber = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.LastFinalizedEpochNumber |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
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
			m.Epochs = append(m.Epochs, EpochEntry{})
			if err := m.Epochs[len(m.Epochs)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Submissions", wireType)
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
			m.Submissions = append(m.Submissions, SubmissionEntry{})
			if err := m.Submissions[len(m.Submissions)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
func (m *EpochEntry) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: EpochEntry: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: EpochEntry: illegal tag %d (wire type %d)", fieldNum, wire)
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
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
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
			if m.Data == nil {
				m.Data = &EpochData{}
			}
			if err := m.Data.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
func (m *SubmissionEntry) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: SubmissionEntry: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SubmissionEntry: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SubmissionKey", wireType)
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
			if m.SubmissionKey == nil {
				m.SubmissionKey = &SubmissionKey{}
			}
			if err := m.SubmissionKey.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
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
			if m.Data == nil {
				m.Data = &SubmissionData{}
			}
			if err := m.Data.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
