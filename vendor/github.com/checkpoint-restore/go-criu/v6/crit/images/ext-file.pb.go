// SPDX-License-Identifier: MIT

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.5
// source: ext-file.proto

package images

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

type ExtFileEntry struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id   *uint32    `protobuf:"varint,1,req,name=id" json:"id,omitempty"`
	Fown *FownEntry `protobuf:"bytes,5,req,name=fown" json:"fown,omitempty"`
}

func (x *ExtFileEntry) Reset() {
	*x = ExtFileEntry{}
	if protoimpl.UnsafeEnabled {
		mi := &file_ext_file_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ExtFileEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExtFileEntry) ProtoMessage() {}

func (x *ExtFileEntry) ProtoReflect() protoreflect.Message {
	mi := &file_ext_file_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExtFileEntry.ProtoReflect.Descriptor instead.
func (*ExtFileEntry) Descriptor() ([]byte, []int) {
	return file_ext_file_proto_rawDescGZIP(), []int{0}
}

func (x *ExtFileEntry) GetId() uint32 {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return 0
}

func (x *ExtFileEntry) GetFown() *FownEntry {
	if x != nil {
		return x.Fown
	}
	return nil
}

var File_ext_file_proto protoreflect.FileDescriptor

var file_ext_file_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x65, 0x78, 0x74, 0x2d, 0x66, 0x69, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x0a, 0x66, 0x6f, 0x77, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x41, 0x0a, 0x0e,
	0x65, 0x78, 0x74, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x0e,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x02, 0x28, 0x0d, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1f,
	0x0a, 0x04, 0x66, 0x6f, 0x77, 0x6e, 0x18, 0x05, 0x20, 0x02, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x66,
	0x6f, 0x77, 0x6e, 0x5f, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x04, 0x66, 0x6f, 0x77, 0x6e,
}

var (
	file_ext_file_proto_rawDescOnce sync.Once
	file_ext_file_proto_rawDescData = file_ext_file_proto_rawDesc
)

func file_ext_file_proto_rawDescGZIP() []byte {
	file_ext_file_proto_rawDescOnce.Do(func() {
		file_ext_file_proto_rawDescData = protoimpl.X.CompressGZIP(file_ext_file_proto_rawDescData)
	})
	return file_ext_file_proto_rawDescData
}

var file_ext_file_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_ext_file_proto_goTypes = []interface{}{
	(*ExtFileEntry)(nil), // 0: ext_file_entry
	(*FownEntry)(nil),    // 1: fown_entry
}
var file_ext_file_proto_depIdxs = []int32{
	1, // 0: ext_file_entry.fown:type_name -> fown_entry
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_ext_file_proto_init() }
func file_ext_file_proto_init() {
	if File_ext_file_proto != nil {
		return
	}
	file_fown_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_ext_file_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ExtFileEntry); i {
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
			RawDescriptor: file_ext_file_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_ext_file_proto_goTypes,
		DependencyIndexes: file_ext_file_proto_depIdxs,
		MessageInfos:      file_ext_file_proto_msgTypes,
	}.Build()
	File_ext_file_proto = out.File
	file_ext_file_proto_rawDesc = nil
	file_ext_file_proto_goTypes = nil
	file_ext_file_proto_depIdxs = nil
}
