package router

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	skr "github.com/solpipe/solpipe-tool/state/staker"
	"github.com/solpipe/solpipe-tool/state/sub"
	val "github.com/solpipe/solpipe-tool/state/validator"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type Router struct {
	ctx context.Context
	//updatePipelineC   chan<- util.ResponseChannel[sgo.PublicKey]
	//updateBidC        chan<- util.ResponseChannel[util.BidForBidder]
	//updateBidSummaryC chan<- util.ResponseChannel[util.BidSummary]
	allGroup   sub.SubscriptionProgramGroup
	internalC  chan<- func(*internal)
	Controller ctr.Controller
	Network    ntk.Network
	oaReq      objReq
	objDelete  objReqClose
}

func CreateRouter(
	ctx context.Context,
	network ntk.Network,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	all *sub.ProgramAllResult,
	version vrs.CbaVersion,
) (Router, error) {
	var err error

	// TODO: add ctx,cancel call here
	startErrorC := make(chan error, 1)
	subErrorC := make(chan error, 1)

	subAll, err := sub.SubscribeProgramAll(ctx, rpcClient, wsClient, subErrorC)
	if err != nil {
		return Router{}, err
	}

	if all == nil {
		all, err = sub.FetchProgramAll(ctx, rpcClient, version)
		if err != nil {
			return Router{}, err
		}
	}
	controller := network.Controller

	validatorG := dssub.SubscriptionRequest(subAll.ValidatorC, func(data sub.ValidatorGroup) bool {
		return true
	})
	pipelineG := dssub.SubscriptionRequest(subAll.PipelineC, func(pg sub.PipelineGroup) bool {
		return true
	})
	refundG := dssub.SubscriptionRequest(subAll.RefundC, func(bl cba.Refunds) bool { return true })
	bidListG := dssub.SubscriptionRequest(subAll.BidListC, func(bl cba.BidList) bool { return true })
	periodRingG := dssub.SubscriptionRequest(subAll.PeriodRingC, func(pr cba.PeriodRing) bool { return true })
	stakerManagerG := dssub.SubscriptionRequest(subAll.StakerManagerC, func(pr sub.StakeGroup) bool { return true })
	stakerReceiptG := dssub.SubscriptionRequest(subAll.StakerReceiptC, func(pr sub.StakerReceiptGroup) bool { return true })
	receiptG := dssub.SubscriptionRequest(subAll.ReceiptC, func(pr sub.ReceiptGroup) bool { return true })
	payoutG := dssub.SubscriptionRequest(subAll.PayoutC, func(pr sub.PayoutWithData) bool { return true })

	oa, oaReq := createObjSub()

	internalC := make(chan func(*internal), 10)

	go loopInternal(
		ctx,
		internalC,
		network,
		controller,
		rpcClient,
		wsClient,
		subErrorC,
		startErrorC,
		oaReq.reqClose,
		validatorG,
		pipelineG,
		refundG,
		bidListG,
		periodRingG,
		stakerManagerG,
		stakerReceiptG,
		receiptG,
		payoutG,
		subAll,
		all,
		oa,
	)
	err = <-startErrorC
	if err != nil {
		return Router{}, err
	}

	return Router{
		ctx:        ctx,
		internalC:  internalC,
		allGroup:   *subAll,
		Controller: controller,
		oaReq:      oaReq,
		objDelete:  oaReq.reqClose,
		Network:    network,
	}, nil
}

type objSub struct {
	controller       *dssub.SubHome[ctr.Controller]
	pipeline         *dssub.SubHome[pipe.Pipeline]
	payout           *dssub.SubHome[pyt.Payout]
	receipt          *dssub.SubHome[rpt.Receipt]
	staker           *dssub.SubHome[skr.Staker]
	validator        *dssub.SubHome[ValidatorWithData]
	controllerCloseC <-chan sgo.PublicKey
	pipelineCloseC   <-chan sgo.PublicKey
	payoutCloseC     <-chan sgo.PublicKey
	receiptCloseC    <-chan sgo.PublicKey
	stakerCloseC     <-chan sgo.PublicKey
	validatorCloseC  <-chan sgo.PublicKey
}

type objReq struct {
	controllerC chan<- dssub.ResponseChannel[ctr.Controller]
	pipelineC   chan<- dssub.ResponseChannel[pipe.Pipeline]
	payoutC     chan<- dssub.ResponseChannel[pyt.Payout]
	receiptC    chan<- dssub.ResponseChannel[rpt.Receipt]
	stakerC     chan<- dssub.ResponseChannel[skr.Staker]
	validatorC  chan<- dssub.ResponseChannel[ValidatorWithData]
	reqClose    objReqClose
}

type objReqClose struct {
	controllerCloseC chan<- sgo.PublicKey
	pipelineCloseC   chan<- sgo.PublicKey
	payoutCloseC     chan<- sgo.PublicKey
	receiptCloseC    chan<- sgo.PublicKey
	stakerCloseC     chan<- sgo.PublicKey
	validatorCloseC  chan<- sgo.PublicKey
}

// After creating a state object, make sure the object is deleted from the router once the state object exists (i.e. the on-chain account being represented has been closed)
func loopDelete(
	ctx context.Context,
	closeC <-chan struct{},
	deleteC chan<- sgo.PublicKey,
	id sgo.PublicKey,
	wsClient *sgows.Client,
) {
	doneC := ctx.Done()
	endSub, err := wsClient.AccountSubscribe(id, sgorpc.CommitmentFinalized)
	if err != nil {
		log.Error(err)
		return
	}
	sendDelete := false
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-endSub.RecvErr():
			log.Error(err)
			sendDelete = false
			break out
		case d := <-endSub.RecvStream():
			x, ok := d.(*sgows.AccountResult)
			if !ok {
				sendDelete = false
				break out
			}
			if x.Value.Lamports == 0 {
				log.Debugf("account id=%s has lamports=0", id.String())
				sendDelete = true
				break out
			}
		case <-closeC:
			break out
		}
	}

	if sendDelete {
		select {
		case <-doneC:
		case deleteC <- id:
			log.Debugf("sending delete for id=%s", id.String())
		}
	}

}

func createObjSub() (*objSub, objReq) {
	oa := new(objSub)
	controllerCloseC := make(chan sgo.PublicKey, 1)
	pipelineCloseC := make(chan sgo.PublicKey, 1)
	payoutCloseC := make(chan sgo.PublicKey, 1)
	receiptCloseC := make(chan sgo.PublicKey, 1)
	stakerCloseC := make(chan sgo.PublicKey, 1)
	validatorCloseC := make(chan sgo.PublicKey, 1)
	oa.controller = dssub.CreateSubHome[ctr.Controller]()
	oa.pipeline = dssub.CreateSubHome[pipe.Pipeline]()
	oa.payout = dssub.CreateSubHome[pyt.Payout]()
	oa.receipt = dssub.CreateSubHome[rpt.Receipt]()
	oa.staker = dssub.CreateSubHome[skr.Staker]()
	oa.validator = dssub.CreateSubHome[ValidatorWithData]()
	oa.controllerCloseC = controllerCloseC
	oa.payoutCloseC = payoutCloseC
	oa.pipelineCloseC = pipelineCloseC
	oa.receiptCloseC = receiptCloseC
	oa.stakerCloseC = stakerCloseC
	oa.validatorCloseC = validatorCloseC
	req := new(objReq)
	req.controllerC = oa.controller.ReqC
	req.pipelineC = oa.pipeline.ReqC
	req.payoutC = oa.payout.ReqC
	req.receiptC = oa.receipt.ReqC
	req.stakerC = oa.staker.ReqC
	req.validatorC = oa.validator.ReqC
	req.reqClose = objReqClose{
		controllerCloseC: controllerCloseC,
		payoutCloseC:     payoutCloseC,
		pipelineCloseC:   pipelineCloseC,
		receiptCloseC:    receiptCloseC,
		stakerCloseC:     stakerCloseC,
		validatorCloseC:  validatorCloseC,
	}

	return oa, *req
}

func (e1 Router) ObjectOnPipeline() dssub.Subscription[pipe.Pipeline] {
	return dssub.SubscriptionRequest(e1.oaReq.pipelineC, func(x pipe.Pipeline) bool { return true })
}

func (e1 Router) ObjectOnPayout() dssub.Subscription[pyt.Payout] {
	return dssub.SubscriptionRequest(e1.oaReq.payoutC, func(x pyt.Payout) bool { return true })
}

func (e1 Router) ObjectOnReceipt() dssub.Subscription[rpt.Receipt] {
	return dssub.SubscriptionRequest(e1.oaReq.receiptC, func(x rpt.Receipt) bool { return true })
}

func (e1 Router) ObjectOnStaker() dssub.Subscription[skr.Staker] {
	return dssub.SubscriptionRequest(e1.oaReq.stakerC, func(x skr.Staker) bool { return true })
}

func (e1 Router) ObjectOnValidator(cb func(ValidatorWithData) bool) dssub.Subscription[ValidatorWithData] {
	if cb == nil {
		cb = func(x ValidatorWithData) bool { return true }
	}
	return dssub.SubscriptionRequest(e1.oaReq.validatorC, cb)
}

func (e1 Router) OnPipeline() dssub.Subscription[sub.PipelineGroup] {
	return dssub.SubscriptionRequest(e1.allGroup.PipelineC, func(pg sub.PipelineGroup) bool { return true })
}

// set pipeline to 0 to receive all periods
func (e1 Router) OnPeriod(pipeline sgo.PublicKey) dssub.Subscription[cba.PeriodRing] {
	return dssub.SubscriptionRequest(e1.allGroup.PeriodRingC, func(pr cba.PeriodRing) bool {
		if pipeline.IsZero() {
			return true
		} else if pr.Pipeline.Equals(pipeline) {
			return true
		}
		return false
	})
}

func (e1 Router) OnBid(bidder sgo.PublicKey) dssub.Subscription[cba.BidList] {
	return dssub.SubscriptionRequest(e1.allGroup.BidListC, func(bg cba.BidList) bool {
		for _, v := range bg.Book {
			if !v.IsBlank {
				if v.User.Equals(bidder) {
					return true
				}
			}
		}
		return false
	})
}

func (e1 Router) OnBidSummary() dssub.Subscription[sub.BidSummary] {
	return dssub.SubscriptionRequest(e1.allGroup.BidSummaryC, func(bg sub.BidSummary) bool {
		return true
	})
}

func (e1 Router) OnStaker() dssub.Subscription[sub.StakeGroup] {
	return dssub.SubscriptionRequest(e1.allGroup.StakerManagerC, func(d sub.StakeGroup) bool {
		return true
	})
}

func (e1 Router) OnReceipt() dssub.Subscription[sub.ReceiptGroup] {
	return dssub.SubscriptionRequest(e1.allGroup.ReceiptC, func(d sub.ReceiptGroup) bool {
		return true
	})
}

func (e1 Router) OnPayout() dssub.Subscription[sub.PayoutWithData] {
	return dssub.SubscriptionRequest(e1.allGroup.PayoutC, func(d sub.PayoutWithData) bool {
		return true
	})
}

func (e1 Router) AddPipeline(id sgo.PublicKey, data cba.Pipeline) error {
	errorC := make(chan error, 1)
	e1.internalC <- func(in *internal) {
		errorC <- in.on_pipeline(sub.PipelineGroup{
			IsOpen: true, Id: id, Data: data,
		}, in.controller.SlotHome())
	}
	return <-errorC
}

func (e1 Router) PayoutByPipelineIdAndStart(pipelineId sgo.PublicKey, start uint64) (pyt.Payout, error) {

	err := e1.ctx.Err()
	if err != nil {
		return pyt.Payout{}, err
	}
	errorC := make(chan error, 1)
	ansC := make(chan pyt.Payout, 1)
	e1.internalC <- func(in *internal) {

		x, present := in.l_payout.byPipeline[pipelineId.String()]
		if !present {
			errorC <- errors.New("no pipeline")
			return
		}
		y, present := x[start]
		if !present {
			errorC <- errors.New("no corresponding period")
			return
		}
		errorC <- nil
		ansC <- y.p
	}
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return pyt.Payout{}, err
	}
	return <-ansC, nil
}

func (e1 Router) PayoutById(id sgo.PublicKey) (pipe.PayoutWithData, error) {

	err := e1.ctx.Err()
	if err != nil {
		return pipe.PayoutWithData{}, err
	}
	errorC := make(chan error, 1)
	ansC := make(chan pipe.PayoutWithData, 1)
	e1.internalC <- func(in *internal) {

		x, present := in.l_payout.byId[id.String()]
		if !present {
			errorC <- errors.New("no pipeline")
			return
		}

		errorC <- nil
		ansC <- pipe.PayoutWithData{
			Id:     id,
			Payout: x.p,
			Data:   x.data,
		}
	}
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return pipe.PayoutWithData{}, err
	}
	return <-ansC, nil
}

func (e1 Router) PipelineById(id sgo.PublicKey) (pipe.Pipeline, error) {
	errorC := make(chan error, 1)
	ansC := make(chan pipe.Pipeline, 1)
	e1.internalC <- func(in *internal) {
		p, present := in.l_pipeline.byId[id.String()]
		if present {
			errorC <- nil
			ansC <- p
		} else {
			errorC <- errors.New("no such pipeline")
		}
	}
	err := <-errorC
	if err != nil {
		return pipe.Pipeline{}, err
	} else {
		return <-ansC, nil
	}
}

func (e1 Router) ValidatorById(id sgo.PublicKey) (val.Validator, error) {
	var err error
	err = e1.ctx.Err()
	if err != nil {
		return val.Validator{}, err
	}
	errorC := make(chan error, 1)
	ansC := make(chan val.Validator, 1)

	e1.internalC <- func(in *internal) {
		p, present := in.l_validator.byId[id.String()]
		if present {
			errorC <- nil
			ansC <- p.v
		} else {
			errorC <- errors.New("no such pipeline")
		}
	}

	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return val.Validator{}, err
	} else {
		return <-ansC, nil
	}
}

func (e1 Router) ValidatorByVote(vote sgo.PublicKey) (val.Validator, error) {
	var err error
	err = e1.ctx.Err()
	if err != nil {
		return val.Validator{}, err
	}
	errorC := make(chan error, 1)
	ansC := make(chan val.Validator, 1)

	e1.internalC <- func(in *internal) {
		p, present := in.l_validator.byVote[vote.String()]
		if present {
			errorC <- nil
			ansC <- p.v
		} else {
			errorC <- errors.New("no such pipeline")
		}
	}

	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return val.Validator{}, err
	} else {
		return <-ansC, nil
	}
}

func (e1 Router) AddBid(list cba.BidList) error {
	errorC := make(chan error, 1)
	e1.internalC <- func(in *internal) {
		errorC <- in.add_bid(list)
	}
	return <-errorC
}

func (e1 Router) OnValidator() dssub.Subscription[sub.ValidatorGroup] {
	return dssub.SubscriptionRequest(e1.allGroup.ValidatorC, func(pg sub.ValidatorGroup) bool { return true })
}

func (e1 Router) AllValidator() ([]val.Validator, error) {
	finishedC := make(chan int, 1)
	ansC := make(chan val.Validator, 10)
	ctx := e1.ctx
	e1.internalC <- func(in *internal) {
		finishedC <- len(in.l_pipeline.byId)
		for _, p := range in.l_validator.byId {
			ansC <- p.v
		}
	}
	doneC := ctx.Done()
	var n int
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case n = <-finishedC:
	}
	list := make([]val.Validator, n)
	for i := 0; i < n; i++ {
		select {
		case <-doneC:
			return nil, errors.New("canceled")
		case x := <-ansC:
			list[i] = x
		}
	}
	return list, nil
}

// with data
func (e1 Router) AllPipeline() ([]pipe.Pipeline, error) {
	finishedC := make(chan int, 1)
	ansC := make(chan pipe.Pipeline, 10)
	ctx := e1.ctx
	e1.internalC <- func(in *internal) {
		finishedC <- len(in.l_pipeline.byId)
		for _, p := range in.l_pipeline.byId {
			ansC <- p
		}
	}
	doneC := ctx.Done()
	var n int
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case n = <-finishedC:
	}
	list := make([]pipe.Pipeline, n)
	for i := 0; i < n; i++ {
		select {
		case <-doneC:
			return nil, errors.New("canceled")
		case x := <-ansC:
			list[i] = x
		}
	}
	return list, nil

}

func (in *internal) add_bid(list cba.BidList) error {
	ref, present := in.l_payout.byId[list.Payout.String()]
	if !present {
		return errors.New("no pipeline")
	}
	ref.p.UpdateBidList(list)

	return nil
}

func (e1 Router) AddPeriod(ring cba.PeriodRing) error {
	errorC := make(chan error, 1)
	e1.internalC <- func(in *internal) {
		errorC <- in.add_period(ring)
	}
	return <-errorC
}

func (in *internal) add_period(ring cba.PeriodRing) error {
	p, present := in.l_pipeline.byId[ring.Pipeline.String()]
	if !present {
		return errors.New("no pipeline")
	}
	p.UpdatePeriod(ring)
	return nil
}
