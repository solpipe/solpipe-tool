package util

import (
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func CopyMessage(a protoreflect.ProtoMessage, b protoreflect.ProtoMessage) {
	data, err := proto.Marshal(a)
	if err != nil {
		panic(err)
	}
	err = proto.Unmarshal(data, b)
	if err != nil {
		panic(err)
	}
}

func CopyValidatorSettings(a *pba.ValidatorSettings) *pba.ValidatorSettings {
	b := new(pba.ValidatorSettings)
	CopyMessage(a, b)
	return b
}

func CopyPeriodSettings(a *pba.PeriodSettings) *pba.PeriodSettings {
	b := new(pba.PeriodSettings)
	CopyMessage(a, b)
	return b
}

func CopyRate(a *pba.Rate) *pba.Rate {
	b := new(pba.Rate)
	CopyMessage(a, b)
	return b
}
