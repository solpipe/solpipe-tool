package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schval "github.com/solpipe/solpipe-tool/scheduler/validator"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type payoutWithPipeline struct {
	pipeline         pipe.Pipeline
	pwd              pipe.PayoutWithData
	pipelineSchedule sch.Schedule
	payoutSchedule   sch.Schedule
}

type payoutInfo struct {
	pwp               payoutWithPipeline
	validatorSchedule sch.Schedule
	cancel            context.CancelFunc
}

func (in *internal) on_payout(pwp payoutWithPipeline) {
	log.Debugf("on payout=%s", pwp.pwd.Id.String())
	_, present := in.payoutM[pwp.pwd.Id.String()]
	if present {
		return
	}
	in.on_pipeline(pwp.pipeline)
	var ctxC context.Context
	finish := pwp.pwd.Data.Period.Start + pwp.pwd.Data.Period.Length - 1
	if finish <= in.latestPeriodFinish {
		log.Debugf("payout=%s finish is too late (%d vs %d)", pwp.pwd.Id.String(), finish, in.latestPeriodFinish)
		return
	}
	in.latestPeriodFinish = finish

	pi := new(payoutInfo)
	in.payoutM[pwp.pwd.Id.String()] = pi
	pi.pwp = pwp
	ctxC, pi.cancel = context.WithCancel(in.ctx)
	pi.validatorSchedule = schval.Schedule(
		ctxC,
		pwp.pwd,
		pwp.pipeline,
		pwp.pipelineSchedule,
		pwp.payoutSchedule,
		in.validator,
	)
	go loopPayout(ctxC, in.eventC, in.errorC, *pi)
}

func loopPayout(
	ctx context.Context,
	eventC chan<- sch.Event,
	errorC chan<- error,
	pi payoutInfo,
) {

	var err error
	doneC := ctx.Done()
	sub := pi.validatorSchedule.OnEvent()
	defer sub.Unsubscribe()
	var event sch.Event
out:
	for {

		select {
		case <-doneC:
			break out
		case err = <-sub.ErrorC:
			break out
		case event = <-sub.StreamC:
			select {
			case <-doneC:
				break out
			case eventC <- event:
			}
		}
	}

	if err != nil {
		errorC <- err
	}
}
