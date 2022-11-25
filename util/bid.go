package util

import (
	pbb "github.com/solpipe/solpipe-tool/proto/bid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func InitStats() *pbb.NetworkStats {
	return &pbb.NetworkStats{
		NetworkTps: 0,
	}
}

func InitBudget() *pbb.TpsBudget {
	return &pbb.TpsBudget{
		MinTps:      0,
		MaxTps:      0,
		MaxSpend:    0,
		MaxBidDelta: 0.25,
	}
}

func CopyProtoMessage[T protoreflect.ProtoMessage](a T, b T) {

	data, err := proto.Marshal(a)
	if err != nil {
		panic(err)
	}
	err = proto.Unmarshal(data, b)
	if err != nil {
		panic(err)
	}
}
