package state

import (
	"errors"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	cba "github.com/solpipe/cba"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"github.com/solpipe/solpipe-tool/state/validator"
	"github.com/solpipe/solpipe-tool/util"
)

type Rate struct {
	N uint64
	D uint64
}

func RateFromProto(r *pba.Rate) Rate {
	return Rate{N: r.Numerator, D: r.Denominator}
}

func (r Rate) Float() (float64, error) {
	if util.MAX_PC < r.N {
		return 0, errors.New("numerator out of range")
	}
	if util.MAX_PC < r.D {
		return 0, errors.New("denominator out of range")
	} else if r.D == 0 {
		return 0, errors.New("divide by zero")
	}
	return float64(r.N) / float64(r.D), nil
}

type PipelineBandwidth struct {
	Tps        uint64
	Validators []validator.Validator
}

func SubscribePipeline(wsClient *sgows.Client) (*sgows.ProgramSubscription, error) {
	return wsClient.ProgramSubscribe(cba.ProgramID, sgorpc.CommitmentFinalized)
	//return wsClient.ProgramSubscribeWithOpts(cba.ProgramID, sgorpc.CommitmentFinalized, sgo.EncodingBase64, []sgorpc.RPCFilter{
	//{
	//DataSize: util.STRUCT_SIZE_PIPELINE,
	//Memcmp: &sgorpc.RPCFilterMemcmp{
	//	Offset: 0,
	//	Bytes:  cba.PipelineDiscriminator[:],
	//},
	//},
	//})
}
