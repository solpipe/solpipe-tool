package state

import (
	cba "github.com/solpipe/cba"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"github.com/solpipe/solpipe-tool/state/validator"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

type Rate struct {
	N uint64
	D uint64
}

func RateFromProto(r *pba.Rate) Rate {
	return Rate{N: r.Numerator, D: r.Denominator}
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
