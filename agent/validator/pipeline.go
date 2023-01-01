package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpyt "github.com/solpipe/solpipe-tool/scheduler/payout"
	schpipe "github.com/solpipe/solpipe-tool/scheduler/pipeline"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	slt "github.com/solpipe/solpipe-tool/state/slot"
)

type pipelineInfo struct {
	p                pipe.Pipeline
	pipelineSchedule sch.Schedule
	cancel           context.CancelFunc
	lookAheadC       chan<- uint64
}

func (in *internal) on_pipeline(p pipe.Pipeline) *pipelineInfo {
	pi, present := in.pipelineM[p.Id.String()]
	if present {
		return pi
	} else {
		pi = new(pipelineInfo)
		in.pipelineM[p.Id.String()] = pi
		pi.p = p
		// we will not do AppendPeriod, so we ignore lookahead
		fakeLookAheadC := make(chan uint64, 1)
		fakeLookAheadC <- 0
		lookAheadC := make(chan uint64, 1)
		pi.lookAheadC = lookAheadC
		pi.pipelineSchedule = schpipe.Schedule(in.ctx, in.router, p, 0, fakeLookAheadC)
		var ctxC context.Context
		ctxC, pi.cancel = context.WithCancel(in.ctx)
		newPayoutC := in.newPayoutC
		go loopPipeline(
			ctxC,
			in.errorC,
			in.router,
			pi.p,
			pi.pipelineSchedule,
			newPayoutC,
			lookAheadC,
		)
		return pi
	}
}

type listenPipelineInternal struct {
	ctx        context.Context
	errorC     chan<- error
	router     rtr.Router
	pipeline   pipe.Pipeline
	ps         sch.Schedule
	newPayoutC chan<- payoutWithPipeline
	sh         slt.SlotHome
	slot       uint64
	lookAhead  uint64
}

// A pipeline has been selected, so listen for new Payout periods.
func loopPipeline(
	ctx context.Context,
	errorC chan<- error,
	router rtr.Router,
	pipeline pipe.Pipeline,
	ps sch.Schedule,
	newPayoutC chan<- payoutWithPipeline,
	lookaheadC <-chan uint64,
) {
	var err error
	doneC := ctx.Done()
	log.Debugf("on listen pipeline=%s", pipeline.Id.String())
	controller := router.Controller
	sh := controller.SlotHome()
	slotSub := sh.OnSlot()
	defer slotSub.Unsubscribe()

	pi := new(listenPipelineInternal)
	pi.ctx = ctx
	pi.router = router
	pi.errorC = errorC
	pi.newPayoutC = newPayoutC
	pi.sh = sh
	pi.pipeline = pipeline
	pi.ps = ps
	pi.lookAhead = 0
	pi.slot = 0

	payoutSub := pi.pipeline.OnPayout()
	defer payoutSub.Unsubscribe()
	newPwdStreamC := make(chan pipe.PayoutWithData)
	go loopPipelineLookupPayout(pi.ctx, pi.errorC, pi.pipeline, newPwdStreamC)
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			slotSub = sh.OnSlot()
			log.Debugf("renewed slot sub for pipeline=%s", pi.pipeline.Id.String())
		case pi.slot = <-slotSub.StreamC:
			//if pi.slot%10 == 0 {
			//	log.Debugf("pipeline=%s slot=%d", pi.pipeline.Id.String(), pi.slot)
			//}
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			pi.on_payout(pwd)
		case pwd := <-newPwdStreamC:
			pi.on_payout(pwd)
		case pi.lookAhead = <-lookaheadC:
			log.Debugf("pipeline=%s updated look ahead=%d", pi.pipeline.Id.String(), pi.lookAhead)
		}
	}

	if err != nil {
		log.Debugf("validator exit: %s", err.Error())
		errorC <- err
	} else {
		log.Debugf("exiting validator listen on pipeline=%s", pi.pipeline.Id.String())
	}
}

func (pi *listenPipelineInternal) on_payout(pwd pipe.PayoutWithData) {

	payoutSchedule := schpyt.Schedule(pi.ctx, pi.router, pwd)
	log.Debugf("on_payout payout=%s", pwd.Id.String())
	doneC := pi.ctx.Done()
	if false {
		//if pi.slot+pi.lookAhead < pwd.Data.Period.Start {
		delta := pwd.Data.Period.Start - (pi.slot + pi.lookAhead)
		log.Debugf("delay is delta=%d for payout=%s", delta, pwd.Id.String())
		go loopDelaySendPayout(
			pi.ctx,
			pi.errorC,
			pi.newPayoutC,
			payoutWithPipeline{
				pwd:              pwd,
				pipeline:         pi.pipeline,
				pipelineSchedule: pi.ps,
				payoutSchedule:   payoutSchedule,
			},
			pi.sh,
			pi.slot,
			pwd.Data.Period.Start,
			pi.lookAhead,
		)
	} else {
		select {
		case <-doneC:
		case pi.newPayoutC <- payoutWithPipeline{
			pwd:              pwd,
			pipeline:         pi.pipeline,
			pipelineSchedule: pi.ps,
			payoutSchedule:   payoutSchedule,
		}:
		}
	}

}

func loopDelaySendPayout(
	ctx context.Context,
	errorC chan<- error,
	newPayoutC chan<- payoutWithPipeline,
	pwp payoutWithPipeline,
	sh slt.SlotHome,
	slot uint64,
	start uint64,
	lookahead uint64,
) {
	var err error
	doneC := ctx.Done()
	slotSub := sh.OnSlot()
out:
	for slot+lookahead < start {
		select {
		case <-doneC:
			return
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
		}
	}
	if err != nil {
		errorC <- err
	} else {
		select {
		case <-doneC:
		case newPayoutC <- pwp:
		}
	}
}

func loopPipelineLookupPayout(
	ctx context.Context,
	errorC chan<- error,
	pipeline pipe.Pipeline,
	pwdC chan<- pipe.PayoutWithData,
) {
	var err error
	doneC := ctx.Done()

	list, err := pipeline.AllPayouts()
	if err != nil {
		select {
		case errorC <- err:
		default:
		}
		return
	}
	for _, pwd := range list {
		select {
		case <-doneC:
			return
		case pwdC <- pwd:
		}
	}
}
