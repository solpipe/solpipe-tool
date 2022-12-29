package validator

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type TriggerValidator struct {
	Context   context.Context
	Validator val.Validator
	Payout    pyt.Payout
	Receipt   rpt.Receipt
	Pipeline  pipe.Pipeline
}

func ReadTrigger(
	event sch.Event,
) (*TriggerValidator, error) {
	if event.Payload == nil {
		return nil, errors.New("no payload")
	}
	d, ok := event.Payload.(*TriggerValidator)
	if !ok {
		return nil, errors.New("bad format for payload")
	}
	return d, nil

}

func (in *internal) on_payout_event(event sch.Event) {
	log.Debugf("on_payout_event: %s", event.String())

	switch event.Type {
	case sch.EVENT_PERIOD_PRE_START:
		log.Debugf("event_period_start payout=%s", in.pwd.Id.String())
	case sch.EVENT_PERIOD_START:
		log.Debugf("event_period_start payout=%s", in.pwd.Id.String())
		in.on_pre_start_kill(event)
	case sch.EVENT_PERIOD_FINISH:
		log.Debugf("event_period_finish payout=%s", in.pwd.Id.String())
		in.on_pre_start_kill(event)
	case sch.EVENT_DELAY_CLOSE_PAYOUT:
		log.Debugf("event_period_post_close payout=%s", in.pwd.Id.String())
		in.on_pre_start_kill(event)
		in.on_post(event)
	default:
		log.Debugf("on_payout_event unknown event: %s", event.String())
	}

}

func (in *internal) on_pre_start_kill(event sch.Event) {
	in.preStartEvent = nil
	if !in.sentPeriodStart {
		in.sentPeriodStart = true
		select {
		case <-in.ctx.Done():
		case in.clockPeriodStartC <- event.IsStateChange:
		}
	}
}

func (in *internal) on_post(event sch.Event) {

	if !in.sentPeriodPostClose {
		in.sentPeriodPostClose = true
		select {
		case <-in.ctx.Done():
		case in.clockPeriodPostCloseC <- event.IsStateChange:
		}
	}
}

// track stakers and validator receipt withdrawal
func (in *internal) on_receipt_event(event sch.Event) {
	log.Debugf("on_receipt_event: %s", event.String())
	switch event.Type {
	case sch.EVENT_STAKER_IS_ADDING:
		in.eventHome.Broadcast(event)
	case sch.EVENT_STAKER_HAVE_WITHDRAWN:
		in.on_staker_have_withdrawn(event)
	case sch.EVENT_STAKER_HAVE_WITHDRAWN_EMPTY:
		in.on_staker_have_withdrawn(event)
	case sch.EVENT_VALIDATOR_HAVE_WITHDRAWN:
		// the validator's relationship with payout is over
		// the receipt account has closed
		if in.cancelValidatorWithraw != nil {
			in.cancelValidatorWithraw()
			in.cancelValidatorWithraw = nil
		}
		in.eventHome.Broadcast(event)
		in.errorC <- nil
	default:
		log.Debugf("on_receipt_event unknown event: %s", event.String())
	}
}

func (in *internal) on_staker_have_withdrawn(event sch.Event) {
	in.eventHome.Broadcast(event)
	log.Debugf("withdrawing validator")
	if in.cancelValidatorWithraw == nil {
		in.ctxValidatorWithraw, in.cancelValidatorWithraw = context.WithCancel(in.ctx)
		in.isValidatorWithdrawTransition = event.IsStateChange
		in.run_validator_withdraw()
	} else {
		in.errorC <- errors.New("duplicate withdraw trigger")
	}
}
