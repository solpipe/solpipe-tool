package pipeline

import (
	"context"
	"errors"
	"fmt"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"github.com/solpipe/solpipe-tool/state"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	"github.com/solpipe/solpipe-tool/state/slot"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type Rate struct {
	N uint64
	D uint64
}

const ALLOTMENT_DEFAULT = 100

type Pipeline struct {
	ctx       context.Context
	cancel    context.CancelFunc
	Id        sgo.PublicKey
	internalC chan<- func(*internal)
	rpc       *sgorpc.Client
	//ws                    *sgows.Client
	slotSub               slot.SlotHome
	updatePipelineC       chan<- dssub.ResponseChannel[cba.Pipeline]
	updatePeriodRingC     chan<- dssub.ResponseChannel[cba.PeriodRing]
	updateClaimC          chan<- dssub.ResponseChannel[cba.Claim]
	updatePayoutC         chan<- dssub.ResponseChannel[PayoutWithData]
	updateValidatorStakeC chan<- validatorStakeUpdate
	updateValidatorC      chan<- dssub.ResponseChannel[ValidatorUpdate]
	updateStakeStatusC    chan<- dssub.ResponseChannel[sub.StakeUpdate]
}

func CreatePipeline(
	ctx context.Context,
	id sgo.PublicKey,
	data *cba.Pipeline,
	rpcClient *sgorpc.Client,
	slotSub slot.SlotHome,
	network ntk.Network,
) (Pipeline, error) {
	var err error
	internalC := make(chan func(*internal), 10)

	pipelineHome := dssub.CreateSubHome[cba.Pipeline]()
	payoutHome := dssub.CreateSubHome[PayoutWithData]()
	periodHome := dssub.CreateSubHome[cba.PeriodRing]()
	claimHome := dssub.CreateSubHome[cba.Claim]()
	validatorHome := dssub.CreateSubHome[ValidatorUpdate]()
	stakeStatusHome := dssub.CreateSubHome[sub.StakeUpdate]()

	pr := new(cba.PeriodRing)
	rf := new(cba.Refunds)

	if data == nil {
		err = rpcClient.GetAccountDataBorshInto(ctx, id, data)
		if err != nil {
			return Pipeline{}, err
		}
	}
	err = rpcClient.GetAccountDataBorshInto(ctx, data.Periods, pr)
	if err != nil {
		return Pipeline{}, err
	}

	err = rpcClient.GetAccountDataBorshInto(ctx, data.Refunds, rf)
	if err != nil {
		return Pipeline{}, err
	}

	updatePeriodRingC := periodHome.ReqC
	updateClaimC := claimHome.ReqC
	updatePayoutC := payoutHome.ReqC
	updatePipelineC := pipelineHome.ReqC
	updateValidatorStakeC := make(chan validatorStakeUpdate, 10)
	updateValidatorC := validatorHome.ReqC
	updateStakeStatusC := stakeStatusHome.ReqC
	ctx2, cancel := context.WithCancel(ctx)
	go loopInternal(
		ctx2,
		cancel,
		internalC,
		updateValidatorStakeC,
		id,
		data,
		pr,
		rf,
		pipelineHome,
		periodHome,
		claimHome,
		payoutHome,
		validatorHome,
		stakeStatusHome,
		network,
	)
	return Pipeline{
		ctx:                ctx2,
		cancel:             cancel,
		Id:                 id,
		internalC:          internalC,
		rpc:                rpcClient,
		slotSub:            slotSub,
		updatePipelineC:    updatePipelineC,
		updatePeriodRingC:  updatePeriodRingC,
		updateClaimC:       updateClaimC,
		updatePayoutC:      updatePayoutC,
		updateValidatorC:   updateValidatorC,
		updateStakeStatusC: updateStakeStatusC,
	}, nil
}

func (e1 Pipeline) PayoutWithData() ([]PayoutWithData, error) {
	doneC := e1.ctx.Done()
	err := e1.ctx.Err()
	if err != nil {
		return nil, err
	}
	ansC := make(chan []PayoutWithData, 1)
	var ans []PayoutWithData
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		ansC <- in.payoutLinkedList.Array()
	}:
	}
	if err != nil {
		return nil, err
	}

	select {
	case <-doneC:
		err = errors.New("canceled")
	case ans = <-ansC:
	}

	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (e1 Pipeline) Data() (cba.Pipeline, error) {
	errorC := make(chan error, 1)
	ansC := make(chan cba.Pipeline, 1)
	e1.internalC <- func(in *internal) {
		if in.data == nil {
			errorC <- errors.New("no data for pipeline")
		} else {
			errorC <- nil
			ansC <- *in.data
		}
	}
	err := <-errorC
	if err != nil {
		return cba.Pipeline{}, err
	}
	return <-ansC, nil
}

func (e1 Pipeline) Close() {
	e1.cancel()
}

func (e1 Pipeline) OnClose() <-chan struct{} {
	return e1.ctx.Done()
}

func convertRateToProto(r *state.Rate) *pba.Rate {
	return &pba.Rate{
		Numerator:   r.N,
		Denominator: r.D,
	}
}

type PipelineSettings struct {
	CrankFee    *state.Rate
	DecayRate   *state.Rate
	PayoutShare *state.Rate
}

func (ps *PipelineSettings) ToProtoRateSettings() *pba.RateSettings {

	return &pba.RateSettings{
		CrankFee:    convertRateToProto(ps.CrankFee),
		DecayRate:   convertRateToProto(ps.DecayRate),
		PayoutShare: convertRateToProto(ps.PayoutShare),
	}
}

func (ps *PipelineSettings) Check() error {

	if ps.CrankFee == nil {
		return errors.New("no crank fee")
	}
	if ps.DecayRate == nil {
		return errors.New("no decay rate")
	}
	if ps.PayoutShare == nil {
		return errors.New("no payout share")
	}

	return nil
}

func (e1 Pipeline) Print() (string, error) {
	data, err := e1.Data()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Pipeline:\n\tCrank fee=%d/%d\n\tValidator Payout Share=%d/%d", data.CrankFeeRate[0], data.CrankFeeRate[1], data.ValidatorPayoutShare[0], data.ValidatorPayoutShare[1]), nil
}

// returns clean linked list reference
func (e1 Pipeline) PeriodRing() (cba.PeriodRing, error) {
	errorC := make(chan error, 1)
	fetchedC := make(chan bool, 1)
	ansC := make(chan cba.PeriodRing, 1)
	rpcClient := e1.rpc
	var err error
	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		if in.data == nil {
			errorC <- errors.New("no pipeline data")
			return
		}
		if in.periods == nil {
			fetchedC <- true
			go loopFetchPeriodRing(in.ctx, rpcClient, in.data.Periods, errorC, ansC)
		} else {
			errorC <- nil
			fetchedC <- false
			ansC <- *in.periods
		}
	}:
	}
	if err != nil {
		return cba.PeriodRing{}, err
	}
	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return cba.PeriodRing{}, err
	}
	ans := <-ansC
	if <-fetchedC {
		e1.UpdatePeriod(ans)
	}

	//a, err := util.GetLinkedListFromPeriodRing(&ans)
	if err != nil {
		return cba.PeriodRing{}, err
	}
	return ans, nil
}

func ConvertPeriodRingToLinkedList(pr cba.PeriodRing) *ll.Generic[cba.PeriodWithPayout] {
	list := ll.CreateGeneric[cba.PeriodWithPayout]()
	var k uint16
	for i := uint16(0); i < pr.Length; i++ {
		k = (i + pr.Start) % uint16(len(pr.Ring))
		list.Append(pr.Ring[k])
	}
	return list
}

func loopFetchPeriodRing(
	ctx context.Context,
	rpcClient *sgorpc.Client,
	pubkey sgo.PublicKey,
	errorC chan<- error,
	ansC chan<- cba.PeriodRing,
) {
	ans := new(cba.PeriodRing)
	err := rpcClient.GetAccountDataBorshInto(ctx, pubkey, ans)
	errorC <- err
	if err != nil {
		ansC <- *ans
	}
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

func (e1 Pipeline) AllPayouts() ([]PayoutWithData, error) {
	doneC := e1.ctx.Done()
	countC := make(chan int)
	payoutC := make(chan PayoutWithData)

	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.payout_list(countC, payoutC)
	}:
	}
	var count int
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case count = <-countC:
	}
	ans := make([]PayoutWithData, count)
	for i := 0; i < count; i++ {
		select {
		case <-doneC:
			return nil, errors.New("canceled")
		case ans[i] = <-payoutC:
		}
	}

	return ans, nil
}

func (in *internal) payout_list(countC chan<- int, payoutC chan<- PayoutWithData) {
	doneC := in.ctx.Done()
	select {
	case <-doneC:
		return
	case countC <- int(in.payoutLinkedList.Size):
	}
	if len(in.payoutById) == 0 {
		return
	}

	err := in.payoutLinkedList.Iterate(func(obj PayoutWithData, index uint32, delete func()) error {
		select {
		case <-doneC:
			return errors.New("canceled")
		case payoutC <- obj:
			return nil
		}
	})
	if err != nil {
		return
	}

}
