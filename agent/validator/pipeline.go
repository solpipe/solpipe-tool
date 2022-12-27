package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpipe "github.com/solpipe/solpipe-tool/scheduler/pipeline"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	slt "github.com/solpipe/solpipe-tool/state/slot"
)

type pipelineInfo struct {
	p          pipe.Pipeline
	ps         sch.Schedule
	cancel     context.CancelFunc
	lookAheadC chan<- uint64
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
		pi.ps = schpipe.Schedule(in.ctx, in.router, p, 0, fakeLookAheadC)
		var ctxC context.Context
		ctxC, pi.cancel = context.WithCancel(in.ctx)
		go loopPipeline(
			ctxC,
			in.errorC,
			in.controller,
			pi.p,
			pi.ps,
			in.newPayoutC,
			lookAheadC,
		)
		return pi
	}
}

type listenPipelineInternal struct {
	ctx        context.Context
	errorC     chan<- error
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
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	ps sch.Schedule,
	newPayoutC chan<- payoutWithPipeline,
	lookaheadC <-chan uint64,
) {
	var err error
	doneC := ctx.Done()
	log.Debugf("on listen pipeline=%s", pipeline.Id.String())

	sh := controller.SlotHome()
	slotSub := sh.OnSlot()
	defer slotSub.Unsubscribe()

	pi := new(listenPipelineInternal)
	pi.ctx = ctx
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

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			slotSub = sh.OnSlot()
		case pi.slot = <-slotSub.StreamC:
			if pi.slot%10 == 0 {
				log.Debugf("pipeline=%s slot=%d", pi.pipeline.Id.String(), pi.slot)
			}
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			pi.on_payout(pwd)
		case pwd := <-newPwdStreamC:
			pi.on_payout(pwd)
		case pi.lookAhead = <-lookaheadC:
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
	log.Debugf("on_payout payout=%s", pwd.Id.String())
	doneC := pi.ctx.Done()
	if pi.slot+pi.lookAhead < pwd.Data.Period.Start {
		delta := pwd.Data.Period.Start - (pi.slot + pi.lookAhead)
		go loopDelaySendPayout(
			pi.ctx,
			pi.errorC,
			pi.newPayoutC,
			payoutWithPipeline{
				pwd:      pwd,
				pipeline: pi.pipeline,
				ps:       pi.ps,
			},
			pi.sh,
			pi.slot+delta,
		)
	} else {
		select {
		case <-doneC:
		case pi.newPayoutC <- payoutWithPipeline{
			pwd:      pwd,
			pipeline: pi.pipeline,
			ps:       pi.ps,
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
	trigger uint64,
) {
	var err error
	var slot uint64
	doneC := ctx.Done()
	slotSub := sh.OnSlot()
out:
	for slot <= trigger {
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
