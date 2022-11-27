// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.3
// source: admin.proto

package admin

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Severity int32

const (
	Severity_DEBUG Severity = 0
	Severity_INFO  Severity = 1
	Severity_ERROR Severity = 2
	Severity_FATAL Severity = 3
)

// Enum value maps for Severity.
var (
	Severity_name = map[int32]string{
		0: "DEBUG",
		1: "INFO",
		2: "ERROR",
		3: "FATAL",
	}
	Severity_value = map[string]int32{
		"DEBUG": 0,
		"INFO":  1,
		"ERROR": 2,
		"FATAL": 3,
	}
)

func (x Severity) Enum() *Severity {
	p := new(Severity)
	*p = x
	return p
}

func (x Severity) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Severity) Descriptor() protoreflect.EnumDescriptor {
	return file_admin_proto_enumTypes[0].Descriptor()
}

func (Severity) Type() protoreflect.EnumType {
	return &file_admin_proto_enumTypes[0]
}

func (x Severity) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Severity.Descriptor instead.
func (Severity) EnumDescriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{0}
}

type Empty struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Empty) Reset() {
	*x = Empty{}
	if protoimpl.UnsafeEnabled {
		mi := &file_admin_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Empty) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Empty) ProtoMessage() {}

func (x *Empty) ProtoReflect() protoreflect.Message {
	mi := &file_admin_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Empty.ProtoReflect.Descriptor instead.
func (*Empty) Descriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{0}
}

type LogLine struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Level   Severity `protobuf:"varint,1,opt,name=level,proto3,enum=admin.Severity" json:"level,omitempty"`
	Message []string `protobuf:"bytes,2,rep,name=message,proto3" json:"message,omitempty"`
}

func (x *LogLine) Reset() {
	*x = LogLine{}
	if protoimpl.UnsafeEnabled {
		mi := &file_admin_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LogLine) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogLine) ProtoMessage() {}

func (x *LogLine) ProtoReflect() protoreflect.Message {
	mi := &file_admin_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogLine.ProtoReflect.Descriptor instead.
func (*LogLine) Descriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{1}
}

func (x *LogLine) GetLevel() Severity {
	if x != nil {
		return x.Level
	}
	return Severity_DEBUG
}

func (x *LogLine) GetMessage() []string {
	if x != nil {
		return x.Message
	}
	return nil
}

type Rate struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Numerator   uint64 `protobuf:"varint,1,opt,name=numerator,proto3" json:"numerator,omitempty"`
	Denominator uint64 `protobuf:"varint,2,opt,name=denominator,proto3" json:"denominator,omitempty"`
}

func (x *Rate) Reset() {
	*x = Rate{}
	if protoimpl.UnsafeEnabled {
		mi := &file_admin_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Rate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Rate) ProtoMessage() {}

func (x *Rate) ProtoReflect() protoreflect.Message {
	mi := &file_admin_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Rate.ProtoReflect.Descriptor instead.
func (*Rate) Descriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{2}
}

func (x *Rate) GetNumerator() uint64 {
	if x != nil {
		return x.Numerator
	}
	return 0
}

func (x *Rate) GetDenominator() uint64 {
	if x != nil {
		return x.Denominator
	}
	return 0
}

type ValidatorSettings struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PipelineId string `protobuf:"bytes,1,opt,name=pipeline_id,json=pipelineId,proto3" json:"pipeline_id,omitempty"`
}

func (x *ValidatorSettings) Reset() {
	*x = ValidatorSettings{}
	if protoimpl.UnsafeEnabled {
		mi := &file_admin_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ValidatorSettings) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ValidatorSettings) ProtoMessage() {}

func (x *ValidatorSettings) ProtoReflect() protoreflect.Message {
	mi := &file_admin_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ValidatorSettings.ProtoReflect.Descriptor instead.
func (*ValidatorSettings) Descriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{3}
}

func (x *ValidatorSettings) GetPipelineId() string {
	if x != nil {
		return x.PipelineId
	}
	return ""
}

type PeriodSettings struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Withhold  uint64 `protobuf:"varint,1,opt,name=withhold,proto3" json:"withhold,omitempty"`
	Lookahead uint64 `protobuf:"varint,2,opt,name=lookahead,proto3" json:"lookahead,omitempty"`
	Length    uint64 `protobuf:"varint,3,opt,name=length,proto3" json:"length,omitempty"`
	TickSize  uint32 `protobuf:"varint,4,opt,name=tick_size,json=tickSize,proto3" json:"tick_size,omitempty"`
}

func (x *PeriodSettings) Reset() {
	*x = PeriodSettings{}
	if protoimpl.UnsafeEnabled {
		mi := &file_admin_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PeriodSettings) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PeriodSettings) ProtoMessage() {}

func (x *PeriodSettings) ProtoReflect() protoreflect.Message {
	mi := &file_admin_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PeriodSettings.ProtoReflect.Descriptor instead.
func (*PeriodSettings) Descriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{4}
}

func (x *PeriodSettings) GetWithhold() uint64 {
	if x != nil {
		return x.Withhold
	}
	return 0
}

func (x *PeriodSettings) GetLookahead() uint64 {
	if x != nil {
		return x.Lookahead
	}
	return 0
}

func (x *PeriodSettings) GetLength() uint64 {
	if x != nil {
		return x.Length
	}
	return 0
}

func (x *PeriodSettings) GetTickSize() uint32 {
	if x != nil {
		return x.TickSize
	}
	return 0
}

type RateSettings struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CrankFee    *Rate `protobuf:"bytes,1,opt,name=crank_fee,json=crankFee,proto3" json:"crank_fee,omitempty"`
	DecayRate   *Rate `protobuf:"bytes,2,opt,name=decay_rate,json=decayRate,proto3" json:"decay_rate,omitempty"`
	PayoutShare *Rate `protobuf:"bytes,3,opt,name=payout_share,json=payoutShare,proto3" json:"payout_share,omitempty"`
}

func (x *RateSettings) Reset() {
	*x = RateSettings{}
	if protoimpl.UnsafeEnabled {
		mi := &file_admin_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RateSettings) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RateSettings) ProtoMessage() {}

func (x *RateSettings) ProtoReflect() protoreflect.Message {
	mi := &file_admin_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RateSettings.ProtoReflect.Descriptor instead.
func (*RateSettings) Descriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{5}
}

func (x *RateSettings) GetCrankFee() *Rate {
	if x != nil {
		return x.CrankFee
	}
	return nil
}

func (x *RateSettings) GetDecayRate() *Rate {
	if x != nil {
		return x.DecayRate
	}
	return nil
}

func (x *RateSettings) GetPayoutShare() *Rate {
	if x != nil {
		return x.PayoutShare
	}
	return nil
}

type TransactionSignature struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Signature string `protobuf:"bytes,1,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (x *TransactionSignature) Reset() {
	*x = TransactionSignature{}
	if protoimpl.UnsafeEnabled {
		mi := &file_admin_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TransactionSignature) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransactionSignature) ProtoMessage() {}

func (x *TransactionSignature) ProtoReflect() protoreflect.Message {
	mi := &file_admin_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransactionSignature.ProtoReflect.Descriptor instead.
func (*TransactionSignature) Descriptor() ([]byte, []int) {
	return file_admin_proto_rawDescGZIP(), []int{6}
}

func (x *TransactionSignature) GetSignature() string {
	if x != nil {
		return x.Signature
	}
	return ""
}

var File_admin_proto protoreflect.FileDescriptor

var file_admin_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x61,
	0x64, 0x6d, 0x69, 0x6e, 0x22, 0x07, 0x0a, 0x05, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x4a, 0x0a,
	0x07, 0x4c, 0x6f, 0x67, 0x4c, 0x69, 0x6e, 0x65, 0x12, 0x25, 0x0a, 0x05, 0x6c, 0x65, 0x76, 0x65,
	0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e,
	0x53, 0x65, 0x76, 0x65, 0x72, 0x69, 0x74, 0x79, 0x52, 0x05, 0x6c, 0x65, 0x76, 0x65, 0x6c, 0x12,
	0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x46, 0x0a, 0x04, 0x52, 0x61, 0x74,
	0x65, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x6e, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x12,
	0x20, 0x0a, 0x0b, 0x64, 0x65, 0x6e, 0x6f, 0x6d, 0x69, 0x6e, 0x61, 0x74, 0x6f, 0x72, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x64, 0x65, 0x6e, 0x6f, 0x6d, 0x69, 0x6e, 0x61, 0x74, 0x6f,
	0x72, 0x22, 0x34, 0x0a, 0x11, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x53, 0x65,
	0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x12, 0x1f, 0x0a, 0x0b, 0x70, 0x69, 0x70, 0x65, 0x6c, 0x69,
	0x6e, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x70, 0x69, 0x70,
	0x65, 0x6c, 0x69, 0x6e, 0x65, 0x49, 0x64, 0x22, 0x7f, 0x0a, 0x0e, 0x50, 0x65, 0x72, 0x69, 0x6f,
	0x64, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x77, 0x69, 0x74,
	0x68, 0x68, 0x6f, 0x6c, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x77, 0x69, 0x74,
	0x68, 0x68, 0x6f, 0x6c, 0x64, 0x12, 0x1c, 0x0a, 0x09, 0x6c, 0x6f, 0x6f, 0x6b, 0x61, 0x68, 0x65,
	0x61, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x6c, 0x6f, 0x6f, 0x6b, 0x61, 0x68,
	0x65, 0x61, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x04, 0x52, 0x06, 0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x12, 0x1b, 0x0a, 0x09, 0x74,
	0x69, 0x63, 0x6b, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x08,
	0x74, 0x69, 0x63, 0x6b, 0x53, 0x69, 0x7a, 0x65, 0x22, 0x94, 0x01, 0x0a, 0x0c, 0x52, 0x61, 0x74,
	0x65, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x12, 0x28, 0x0a, 0x09, 0x63, 0x72, 0x61,
	0x6e, 0x6b, 0x5f, 0x66, 0x65, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x61,
	0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x52, 0x61, 0x74, 0x65, 0x52, 0x08, 0x63, 0x72, 0x61, 0x6e, 0x6b,
	0x46, 0x65, 0x65, 0x12, 0x2a, 0x0a, 0x0a, 0x64, 0x65, 0x63, 0x61, 0x79, 0x5f, 0x72, 0x61, 0x74,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e,
	0x52, 0x61, 0x74, 0x65, 0x52, 0x09, 0x64, 0x65, 0x63, 0x61, 0x79, 0x52, 0x61, 0x74, 0x65, 0x12,
	0x2e, 0x0a, 0x0c, 0x70, 0x61, 0x79, 0x6f, 0x75, 0x74, 0x5f, 0x73, 0x68, 0x61, 0x72, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x52, 0x61,
	0x74, 0x65, 0x52, 0x0b, 0x70, 0x61, 0x79, 0x6f, 0x75, 0x74, 0x53, 0x68, 0x61, 0x72, 0x65, 0x22,
	0x34, 0x0a, 0x14, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x69,
	0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61,
	0x74, 0x75, 0x72, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x2a, 0x35, 0x0a, 0x08, 0x53, 0x65, 0x76, 0x65, 0x72, 0x69, 0x74,
	0x79, 0x12, 0x09, 0x0a, 0x05, 0x44, 0x45, 0x42, 0x55, 0x47, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04,
	0x49, 0x4e, 0x46, 0x4f, 0x10, 0x01, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10,
	0x02, 0x12, 0x09, 0x0a, 0x05, 0x46, 0x41, 0x54, 0x41, 0x4c, 0x10, 0x03, 0x32, 0xb9, 0x01, 0x0a,
	0x09, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x12, 0x36, 0x0a, 0x0a, 0x47, 0x65,
	0x74, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x12, 0x0c, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x18, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x56,
	0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73,
	0x22, 0x00, 0x12, 0x42, 0x0a, 0x0a, 0x53, 0x65, 0x74, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74,
	0x12, 0x18, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74,
	0x6f, 0x72, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x1a, 0x18, 0x2e, 0x61, 0x64, 0x6d,
	0x69, 0x6e, 0x2e, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x53, 0x65, 0x74, 0x74,
	0x69, 0x6e, 0x67, 0x73, 0x22, 0x00, 0x12, 0x30, 0x0a, 0x0c, 0x47, 0x65, 0x74, 0x4c, 0x6f, 0x67,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x0c, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x45,
	0x6d, 0x70, 0x74, 0x79, 0x1a, 0x0e, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x4c, 0x6f, 0x67,
	0x4c, 0x69, 0x6e, 0x65, 0x22, 0x00, 0x30, 0x01, 0x32, 0xe4, 0x01, 0x0a, 0x08, 0x50, 0x69, 0x70,
	0x65, 0x6c, 0x69, 0x6e, 0x65, 0x12, 0x32, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x50, 0x65, 0x72, 0x69,
	0x6f, 0x64, 0x12, 0x0c, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79,
	0x1a, 0x15, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x53,
	0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x22, 0x00, 0x12, 0x2f, 0x0a, 0x08, 0x47, 0x65, 0x74,
	0x52, 0x61, 0x74, 0x65, 0x73, 0x12, 0x0c, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x45, 0x6d,
	0x70, 0x74, 0x79, 0x1a, 0x13, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x52, 0x61, 0x74, 0x65,
	0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x22, 0x00, 0x12, 0x36, 0x0a, 0x08, 0x53, 0x65,
	0x74, 0x52, 0x61, 0x74, 0x65, 0x73, 0x12, 0x13, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x52,
	0x61, 0x74, 0x65, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x1a, 0x13, 0x2e, 0x61, 0x64,
	0x6d, 0x69, 0x6e, 0x2e, 0x52, 0x61, 0x74, 0x65, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73,
	0x22, 0x00, 0x12, 0x3b, 0x0a, 0x09, 0x53, 0x65, 0x74, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x12,
	0x15, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x53, 0x65,
	0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x1a, 0x15, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x50,
	0x65, 0x72, 0x69, 0x6f, 0x64, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x73, 0x22, 0x00, 0x42,
	0x2d, 0x5a, 0x2b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x6f,
	0x6c, 0x6d, 0x61, 0x74, 0x65, 0x44, 0x65, 0x76, 0x2f, 0x67, 0x6f, 0x2d, 0x73, 0x74, 0x61, 0x6b,
	0x65, 0x72, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_admin_proto_rawDescOnce sync.Once
	file_admin_proto_rawDescData = file_admin_proto_rawDesc
)

func file_admin_proto_rawDescGZIP() []byte {
	file_admin_proto_rawDescOnce.Do(func() {
		file_admin_proto_rawDescData = protoimpl.X.CompressGZIP(file_admin_proto_rawDescData)
	})
	return file_admin_proto_rawDescData
}

var file_admin_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_admin_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_admin_proto_goTypes = []interface{}{
	(Severity)(0),                // 0: admin.Severity
	(*Empty)(nil),                // 1: admin.Empty
	(*LogLine)(nil),              // 2: admin.LogLine
	(*Rate)(nil),                 // 3: admin.Rate
	(*ValidatorSettings)(nil),    // 4: admin.ValidatorSettings
	(*PeriodSettings)(nil),       // 5: admin.PeriodSettings
	(*RateSettings)(nil),         // 6: admin.RateSettings
	(*TransactionSignature)(nil), // 7: admin.TransactionSignature
}
var file_admin_proto_depIdxs = []int32{
	0,  // 0: admin.LogLine.level:type_name -> admin.Severity
	3,  // 1: admin.RateSettings.crank_fee:type_name -> admin.Rate
	3,  // 2: admin.RateSettings.decay_rate:type_name -> admin.Rate
	3,  // 3: admin.RateSettings.payout_share:type_name -> admin.Rate
	1,  // 4: admin.Validator.GetDefault:input_type -> admin.Empty
	4,  // 5: admin.Validator.SetDefault:input_type -> admin.ValidatorSettings
	1,  // 6: admin.Validator.GetLogStream:input_type -> admin.Empty
	1,  // 7: admin.Pipeline.GetPeriod:input_type -> admin.Empty
	1,  // 8: admin.Pipeline.GetRates:input_type -> admin.Empty
	6,  // 9: admin.Pipeline.SetRates:input_type -> admin.RateSettings
	5,  // 10: admin.Pipeline.SetPeriod:input_type -> admin.PeriodSettings
	4,  // 11: admin.Validator.GetDefault:output_type -> admin.ValidatorSettings
	4,  // 12: admin.Validator.SetDefault:output_type -> admin.ValidatorSettings
	2,  // 13: admin.Validator.GetLogStream:output_type -> admin.LogLine
	5,  // 14: admin.Pipeline.GetPeriod:output_type -> admin.PeriodSettings
	6,  // 15: admin.Pipeline.GetRates:output_type -> admin.RateSettings
	6,  // 16: admin.Pipeline.SetRates:output_type -> admin.RateSettings
	5,  // 17: admin.Pipeline.SetPeriod:output_type -> admin.PeriodSettings
	11, // [11:18] is the sub-list for method output_type
	4,  // [4:11] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_admin_proto_init() }
func file_admin_proto_init() {
	if File_admin_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_admin_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Empty); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_admin_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LogLine); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_admin_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Rate); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_admin_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ValidatorSettings); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_admin_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PeriodSettings); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_admin_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RateSettings); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_admin_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TransactionSignature); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_admin_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_admin_proto_goTypes,
		DependencyIndexes: file_admin_proto_depIdxs,
		EnumInfos:         file_admin_proto_enumTypes,
		MessageInfos:      file_admin_proto_msgTypes,
	}.Build()
	File_admin_proto = out.File
	file_admin_proto_rawDesc = nil
	file_admin_proto_goTypes = nil
	file_admin_proto_depIdxs = nil
}
