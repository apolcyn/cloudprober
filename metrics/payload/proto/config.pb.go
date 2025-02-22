// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v3.21.5
// source: github.com/cloudprober/cloudprober/metrics/payload/proto/config.proto

package proto

import (
	proto "github.com/cloudprober/cloudprober/metrics/proto"
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

// MetricsKind specifies whether to treat output metrics as GAUGE or
// CUMULATIVE. If left unspecified, metrics from ONCE mode probes are treated
// as GAUGE and metrics from SERVER mode probes are treated as CUMULATIVE.
type OutputMetricsOptions_MetricsKind int32

const (
	OutputMetricsOptions_UNDEFINED  OutputMetricsOptions_MetricsKind = 0
	OutputMetricsOptions_GAUGE      OutputMetricsOptions_MetricsKind = 1
	OutputMetricsOptions_CUMULATIVE OutputMetricsOptions_MetricsKind = 2
)

// Enum value maps for OutputMetricsOptions_MetricsKind.
var (
	OutputMetricsOptions_MetricsKind_name = map[int32]string{
		0: "UNDEFINED",
		1: "GAUGE",
		2: "CUMULATIVE",
	}
	OutputMetricsOptions_MetricsKind_value = map[string]int32{
		"UNDEFINED":  0,
		"GAUGE":      1,
		"CUMULATIVE": 2,
	}
)

func (x OutputMetricsOptions_MetricsKind) Enum() *OutputMetricsOptions_MetricsKind {
	p := new(OutputMetricsOptions_MetricsKind)
	*p = x
	return p
}

func (x OutputMetricsOptions_MetricsKind) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (OutputMetricsOptions_MetricsKind) Descriptor() protoreflect.EnumDescriptor {
	return file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_enumTypes[0].Descriptor()
}

func (OutputMetricsOptions_MetricsKind) Type() protoreflect.EnumType {
	return &file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_enumTypes[0]
}

func (x OutputMetricsOptions_MetricsKind) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Do not use.
func (x *OutputMetricsOptions_MetricsKind) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = OutputMetricsOptions_MetricsKind(num)
	return nil
}

// Deprecated: Use OutputMetricsOptions_MetricsKind.Descriptor instead.
func (OutputMetricsOptions_MetricsKind) EnumDescriptor() ([]byte, []int) {
	return file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescGZIP(), []int{0, 0}
}

type OutputMetricsOptions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MetricsKind *OutputMetricsOptions_MetricsKind `protobuf:"varint,1,opt,name=metrics_kind,json=metricsKind,enum=cloudprober.metrics.payload.OutputMetricsOptions_MetricsKind" json:"metrics_kind,omitempty"`
	// Additional labels (comma-separated) to attach to the output metrics, e.g.
	// "region=us-east1,zone=us-east1-d". ptype="external" and probe="<probeName>"
	// are attached automatically.
	AdditionalLabels *string `protobuf:"bytes,2,opt,name=additional_labels,json=additionalLabels" json:"additional_labels,omitempty"`
	// Whether to aggregate metrics in Cloudprober. If enabled, Cloudprober
	// aggregates the metrics returned by the external probe process -- external
	// probe process should return metrics only since the last probe run.
	// Note that this option is mutually exclusive with GAUGE metrics and
	// cloudprober will fail during initialization if both options are enabled.
	AggregateInCloudprober *bool `protobuf:"varint,3,opt,name=aggregate_in_cloudprober,json=aggregateInCloudprober,def=0" json:"aggregate_in_cloudprober,omitempty"`
	// Metrics that should be treated as distributions. These metrics are exported
	// by the external probe program as comma-separated list of values, for
	// example: "op_latency 4.7,5.6,5.9,6.1,4.9". To be able to build distribution
	// from these values, these metrics should be pre-configured in external
	// probe:
	//
	//	dist_metric {
	//	  key: "op_latency"
	//	  value {
	//	    explicit_buckets: "1,2,4,8,16,32,64,128,256"
	//	  }
	//	}
	DistMetric map[string]*proto.Dist `protobuf:"bytes,4,rep,name=dist_metric,json=distMetric" json:"dist_metric,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
}

// Default values for OutputMetricsOptions fields.
const (
	Default_OutputMetricsOptions_AggregateInCloudprober = bool(false)
)

func (x *OutputMetricsOptions) Reset() {
	*x = OutputMetricsOptions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *OutputMetricsOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OutputMetricsOptions) ProtoMessage() {}

func (x *OutputMetricsOptions) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OutputMetricsOptions.ProtoReflect.Descriptor instead.
func (*OutputMetricsOptions) Descriptor() ([]byte, []int) {
	return file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescGZIP(), []int{0}
}

func (x *OutputMetricsOptions) GetMetricsKind() OutputMetricsOptions_MetricsKind {
	if x != nil && x.MetricsKind != nil {
		return *x.MetricsKind
	}
	return OutputMetricsOptions_UNDEFINED
}

func (x *OutputMetricsOptions) GetAdditionalLabels() string {
	if x != nil && x.AdditionalLabels != nil {
		return *x.AdditionalLabels
	}
	return ""
}

func (x *OutputMetricsOptions) GetAggregateInCloudprober() bool {
	if x != nil && x.AggregateInCloudprober != nil {
		return *x.AggregateInCloudprober
	}
	return Default_OutputMetricsOptions_AggregateInCloudprober
}

func (x *OutputMetricsOptions) GetDistMetric() map[string]*proto.Dist {
	if x != nil {
		return x.DistMetric
	}
	return nil
}

var File_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto protoreflect.FileDescriptor

var file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDesc = []byte{
	0x0a, 0x45, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6c, 0x6f,
	0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x2f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72,
	0x6f, 0x62, 0x65, 0x72, 0x2f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2f, 0x70, 0x61, 0x79,
	0x6c, 0x6f, 0x61, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1b, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72,
	0x6f, 0x62, 0x65, 0x72, 0x2e, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x70, 0x61, 0x79,
	0x6c, 0x6f, 0x61, 0x64, 0x1a, 0x3b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x2f, 0x63, 0x6c, 0x6f,
	0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x2f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x64, 0x69, 0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0xdd, 0x03, 0x0a, 0x14, 0x4f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x60, 0x0a, 0x0c, 0x6d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x5f, 0x6b, 0x69, 0x6e, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x3d, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x2e, 0x6d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x2e, 0x4f,
	0x75, 0x74, 0x70, 0x75, 0x74, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x4f, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x2e, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x4b, 0x69, 0x6e, 0x64, 0x52,
	0x0b, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x4b, 0x69, 0x6e, 0x64, 0x12, 0x2b, 0x0a, 0x11,
	0x61, 0x64, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x5f, 0x6c, 0x61, 0x62, 0x65, 0x6c,
	0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x61, 0x64, 0x64, 0x69, 0x74, 0x69, 0x6f,
	0x6e, 0x61, 0x6c, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x3f, 0x0a, 0x18, 0x61, 0x67, 0x67,
	0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x5f, 0x69, 0x6e, 0x5f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70,
	0x72, 0x6f, 0x62, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x3a, 0x05, 0x66, 0x61, 0x6c,
	0x73, 0x65, 0x52, 0x16, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x49, 0x6e, 0x43,
	0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x12, 0x62, 0x0a, 0x0b, 0x64, 0x69,
	0x73, 0x74, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x41, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x2e, 0x6d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x2e, 0x4f, 0x75,
	0x74, 0x70, 0x75, 0x74, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x4f, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x2e, 0x44, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x52, 0x0a, 0x64, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x1a, 0x58,
	0x0a, 0x0f, 0x44, 0x69, 0x73, 0x74, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03,
	0x6b, 0x65, 0x79, 0x12, 0x2f, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x19, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72,
	0x2e, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x44, 0x69, 0x73, 0x74, 0x52, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x37, 0x0a, 0x0b, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x4b, 0x69, 0x6e, 0x64, 0x12, 0x0d, 0x0a, 0x09, 0x55, 0x4e, 0x44, 0x45, 0x46,
	0x49, 0x4e, 0x45, 0x44, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x47, 0x41, 0x55, 0x47, 0x45, 0x10,
	0x01, 0x12, 0x0e, 0x0a, 0x0a, 0x43, 0x55, 0x4d, 0x55, 0x4c, 0x41, 0x54, 0x49, 0x56, 0x45, 0x10,
	0x02, 0x42, 0x3a, 0x5a, 0x38, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x63, 0x6c, 0x6f, 0x75, 0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x2f, 0x63, 0x6c, 0x6f, 0x75,
	0x64, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x72, 0x2f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2f,
	0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
}

var (
	file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescOnce sync.Once
	file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescData = file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDesc
)

func file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescGZIP() []byte {
	file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescOnce.Do(func() {
		file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescData)
	})
	return file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDescData
}

var file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_goTypes = []interface{}{
	(OutputMetricsOptions_MetricsKind)(0), // 0: cloudprober.metrics.payload.OutputMetricsOptions.MetricsKind
	(*OutputMetricsOptions)(nil),          // 1: cloudprober.metrics.payload.OutputMetricsOptions
	nil,                                   // 2: cloudprober.metrics.payload.OutputMetricsOptions.DistMetricEntry
	(*proto.Dist)(nil),                    // 3: cloudprober.metrics.Dist
}
var file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_depIdxs = []int32{
	0, // 0: cloudprober.metrics.payload.OutputMetricsOptions.metrics_kind:type_name -> cloudprober.metrics.payload.OutputMetricsOptions.MetricsKind
	2, // 1: cloudprober.metrics.payload.OutputMetricsOptions.dist_metric:type_name -> cloudprober.metrics.payload.OutputMetricsOptions.DistMetricEntry
	3, // 2: cloudprober.metrics.payload.OutputMetricsOptions.DistMetricEntry.value:type_name -> cloudprober.metrics.Dist
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_init() }
func file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_init() {
	if File_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*OutputMetricsOptions); i {
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
			RawDescriptor: file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_goTypes,
		DependencyIndexes: file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_depIdxs,
		EnumInfos:         file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_enumTypes,
		MessageInfos:      file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_msgTypes,
	}.Build()
	File_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto = out.File
	file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_rawDesc = nil
	file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_goTypes = nil
	file_github_com_cloudprober_cloudprober_metrics_payload_proto_config_proto_depIdxs = nil
}
